// Numpad entry screen (U5). Amount is accumulated in integer cents as digits are
// pressed (calculator-style), so 1-2-5-0 becomes 12.50. Tapping a category
// commits; a lost connection shows a Retry that reuses one idempotency key so a
// re-send cannot double-submit.
(function () {
  "use strict";

  var MAX_CENTS = 99999999; // €999,999.99 cap
  var amountCents = 0;
  var inflight = false;
  var pendingKey = null; // reused across a retry so the server dedupes
  var lastCommit = null; // { key, btn } to replay on Retry

  var amountEl = document.getElementById("amount");
  var errorEl = document.getElementById("error");
  var errorText = document.getElementById("error-text");
  var retryBtn = document.getElementById("retry");
  var noteEl = document.getElementById("note");
  var dateEl = document.getElementById("date");
  var extra = document.getElementById("extra");
  var extraToggle = document.getElementById("extra-toggle");
  var cats = Array.prototype.slice.call(document.querySelectorAll(".cat"));

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

  function pressDigit(d) {
    var next = amountCents * 10 + d;
    if (next > MAX_CENTS) return;
    amountCents = next;
    render();
  }

  function backspace() {
    amountCents = Math.floor(amountCents / 10);
    render();
  }

  function resetAfterCommit() {
    amountCents = 0;
    noteEl.value = "";
    dateEl.value = todayLocal();
    collapseExtra();
    pendingKey = null;
    lastCommit = null;
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

  function commit(key, btn) {
    if (amountCents <= 0 || inflight) return;
    if (!pendingKey) {
      pendingKey =
        (window.crypto && crypto.randomUUID && crypto.randomUUID()) ||
        String(Date.now()) + "-" + Math.random().toString(16).slice(2);
    }
    lastCommit = { key: key, btn: btn };
    inflight = true;
    setCatsDisabled(true);

    fetch("/api/expenses", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": pendingKey,
      },
      body: JSON.stringify({
        amount_cents: amountCents,
        category_key: key,
        note: noteEl.value.trim(),
        spent_on: dateEl.value || todayLocal(),
      }),
    })
      .then(function (res) {
        if (res.ok) {
          flashConfirm(btn);
          resetAfterCommit();
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
    if (lastCommit) commit(lastCommit.key, lastCommit.btn);
  });

  // Desktop: physical keyboard drives the same accumulator.
  document.addEventListener("keydown", function (e) {
    if (e.key >= "0" && e.key <= "9") {
      pressDigit(parseInt(e.key, 10));
    } else if (e.key === "Backspace" && document.activeElement !== noteEl) {
      e.preventDefault();
      backspace();
    }
  });

  dateEl.value = todayLocal();
  render();
})();
