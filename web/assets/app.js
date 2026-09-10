// Numpad entry screen (U5). Amount is accumulated in integer cents as digits are
// pressed (calculator-style), so 1-2-5-0 becomes 12.50. Tapping a category
// commits; a lost connection shows a Retry that reuses one idempotency key so a
// re-send cannot double-submit.
(function () {
  "use strict";

  var MAX_CENTS = 99999999; // €999,999.99 cap
  var amountCents = 0;
  var inflight = false;
  var dateTouched = false; // true once the user picks a date, so we stop tracking "today"
  var lastAttempt = null; // { payload, key, btn } — for an exact Retry of a failed send

  function newKey() {
    return (
      (window.crypto && crypto.randomUUID && crypto.randomUUID()) ||
      String(Date.now()) + "-" + Math.random().toString(16).slice(2)
    );
  }

  var amountEl = document.getElementById("amount");
  var errorEl = document.getElementById("error");
  var errorText = document.getElementById("error-text");
  var retryBtn = document.getElementById("retry");
  var noteEl = document.getElementById("note");
  var dateEl = document.getElementById("date");
  var extra = document.getElementById("extra");
  var extraToggle = document.getElementById("extra-toggle");
  var cats = Array.prototype.slice.call(document.querySelectorAll(".cat"));
  var recentEl = document.getElementById("recent");
  var recentListEl = document.getElementById("recent-list");

  // Category icon/label for the recent list, read straight off the buttons the
  // server already rendered — no second source of truth to keep in sync.
  var catInfo = {};
  cats.forEach(function (b) {
    catInfo[b.getAttribute("data-key")] = {
      icon: b.querySelector(".cat-icon").textContent,
      label: b.querySelector(".cat-label").textContent,
    };
  });

  function esc(s) {
    return String(s).replace(/[&<>"]/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c];
    });
  }

  function todayLocal() {
    var d = new Date();
    var m = String(d.getMonth() + 1).padStart(2, "0");
    var day = String(d.getDate()).padStart(2, "0");
    return d.getFullYear() + "-" + m + "-" + day;
  }

  function render() {
    var euros = (amountCents / 100).toFixed(2);
    amountEl.innerHTML = "€&nbsp;" + euros;
  }

  function monthOf(dateStr) {
    return dateStr.slice(0, 7);
  }

  // "today" / "yesterday" for the two common cases, the plain date otherwise —
  // the recent list is a quick glance, not a ledger.
  function relDate(spentOn) {
    if (spentOn === todayLocal()) return "today";
    var d = new Date();
    d.setDate(d.getDate() - 1);
    var m = String(d.getMonth() + 1).padStart(2, "0");
    var day = String(d.getDate()).padStart(2, "0");
    if (spentOn === d.getFullYear() + "-" + m + "-" + day) return "yesterday";
    return spentOn;
  }

  function renderRecent(list) {
    var rows = list.slice(0, 5);
    if (!rows.length) {
      recentEl.hidden = true;
      return;
    }
    recentListEl.innerHTML = rows
      .map(function (e) {
        var c = catInfo[e.category_key] || { icon: "", label: e.category_key };
        var note = e.note ? '<span class="rnote">' + esc(e.note) + "</span>" : "";
        return (
          '<div class="rrow">' +
          '<span class="cat-icon" aria-hidden="true">' + esc(c.icon) + "</span>" +
          '<span class="rmain"><span class="rcat">' + esc(c.label) + "</span>" + note + "</span>" +
          '<span class="rmeta"><span class="ramt">€&nbsp;' + (e.amount_cents / 100).toFixed(2) + "</span>" +
          '<span class="rdate">' + esc(relDate(e.spent_on)) + "</span></span>" +
          "</div>"
        );
      })
      .join("");
    recentEl.hidden = false;
  }

  // Load the most recent entries. The current month usually has them; early in a
  // month it may not, so fall back to the previous month to fill the list.
  function loadRecent() {
    var thisMonth = monthOf(todayLocal());
    fetch("/api/expenses?month=" + thisMonth)
      .then(function (r) { return r.ok ? r.json() : []; })
      .then(function (list) {
        if (list.length >= 5) {
          renderRecent(list);
          return;
        }
        var d = new Date();
        d.setDate(1);
        d.setMonth(d.getMonth() - 1);
        var prev = d.getFullYear() + "-" + String(d.getMonth() + 1).padStart(2, "0");
        fetch("/api/expenses?month=" + prev)
          .then(function (r) { return r.ok ? r.json() : []; })
          .then(function (older) { renderRecent(list.concat(older)); })
          .catch(function () { renderRecent(list); });
      })
      .catch(function () {});
  }

  function pressDigit(d) {
    var next = amountCents * 10 + d;
    if (next > MAX_CENTS) return;
    amountCents = next;
    dropPendingRetry();
    render();
  }

  function backspace() {
    amountCents = Math.floor(amountCents / 10);
    dropPendingRetry();
    render();
  }

  // A failed send leaves the amount on screen next to a Retry button. Once the
  // user edits anything, that captured payload no longer matches what they see,
  // so retrying it would silently save the wrong entry — drop it instead.
  function dropPendingRetry() {
    if (!lastAttempt || inflight) return;
    lastAttempt = null;
    hideError();
  }

  function resetAfterCommit() {
    amountCents = 0;
    noteEl.value = "";
    dateEl.value = todayLocal();
    dateTouched = false;
    collapseExtra();
    lastAttempt = null;
    hideError();
    render();
  }

  function showError(msg) {
    errorText.textContent = msg;
    errorEl.hidden = false;
  }
  function hideError() {
    errorEl.hidden = true;
  }

  function flashConfirm(btn) {
    btn.classList.add("confirmed");
    setTimeout(function () {
      btn.classList.remove("confirmed");
    }, 800);
  }

  function setCatsDisabled(disabled) {
    cats.forEach(function (b) {
      b.disabled = disabled;
    });
  }

  // commit captures the current amount/note/date and a fresh idempotency key,
  // then sends. Each category tap is its own capture with its own key, so an
  // earlier failed attempt can never dedupe a later, different entry.
  function commit(catKey, btn) {
    if (amountCents <= 0 || inflight) return;
    lastAttempt = {
      payload: {
        amount_cents: amountCents,
        category_key: catKey,
        note: noteEl.value.trim(),
        // Recomputed per commit: an installed PWA is usually resumed, not
        // reloaded, so a date fixed at load time would still say "yesterday"
        // after midnight and file the expense in the wrong day — or month.
        spent_on: dateTouched && dateEl.value ? dateEl.value : todayLocal(),
      },
      key: newKey(),
      btn: btn,
    };
    send();
  }

  // send transmits lastAttempt. Retry calls it again with the same payload and
  // key, so a re-send after a lost response cannot double-submit.
  function send() {
    inflight = true;
    setCatsDisabled(true);

    fetch("/api/expenses", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": lastAttempt.key,
      },
      body: JSON.stringify(lastAttempt.payload),
    })
      .then(function (res) {
        if (res.ok) {
          flashConfirm(lastAttempt.btn);
          resetAfterCommit();
          loadRecent();
        } else if (res.status === 401) {
          window.location.href = "/login";
        } else {
          showError("Could not save. Try again.");
        }
      })
      .catch(function () {
        // Network/offline: keep the amount + category, offer Retry.
        showError("No connection — not saved.");
      })
      .finally(function () {
        inflight = false;
        setCatsDisabled(false);
      });
  }

  function collapseExtra() {
    extra.hidden = true;
    extraToggle.setAttribute("aria-expanded", "false");
  }

  // --- wiring ---

  document.querySelectorAll(".key[data-digit]").forEach(function (b) {
    b.addEventListener("click", function () {
      pressDigit(parseInt(b.getAttribute("data-digit"), 10));
    });
  });
  document.getElementById("backspace").addEventListener("click", backspace);

  cats.forEach(function (b) {
    b.addEventListener("click", function () {
      commit(b.getAttribute("data-key"), b);
    });
  });

  extraToggle.addEventListener("click", function () {
    var open = extra.hidden;
    extra.hidden = !open;
    extraToggle.setAttribute("aria-expanded", String(open));
  });

  retryBtn.addEventListener("click", function () {
    if (lastAttempt && !inflight) send();
  });

  noteEl.addEventListener("input", dropPendingRetry);
  dateEl.addEventListener("input", function () {
    dateTouched = true;
    dropPendingRetry();
  });

  // Resuming a frozen PWA the next day must move the untouched date field on.
  document.addEventListener("visibilitychange", function () {
    if (!document.hidden && !dateTouched) dateEl.value = todayLocal();
  });

  // Desktop: physical keyboard drives the same accumulator — but never while a
  // text/date/select field is focused, so typing a note doesn't touch the amount.
  document.addEventListener("keydown", function (e) {
    var el = document.activeElement;
    if (el && (el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.tagName === "SELECT")) {
      return;
    }
    if (e.key >= "0" && e.key <= "9") {
      pressDigit(parseInt(e.key, 10));
    } else if (e.key === "Backspace") {
      e.preventDefault();
      backspace();
    }
  });

  dateEl.value = todayLocal();
  render();
  loadRecent();
})();
