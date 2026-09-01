// Review surfaces (U6) + edit/delete (U7). Shows a month's total, per-category
// breakdown, and history, with prev/next month navigation. Tapping a history row
// opens an editor; delete requires a confirm.
(function () {
  "use strict";

  var cats = window.__CATEGORIES__ || [];
  var catMap = {};
  cats.forEach(function (c) { catMap[c.key] = c; });

  var byId = {}; // id -> expense, from the current history render
  var editingId = null;

  var monthLabelEl = document.getElementById("month-label");
  var totalEl = document.getElementById("total");
  var breakdownEl = document.getElementById("breakdown");
  var historyEl = document.getElementById("history");

  var editBg = document.getElementById("editbg");
  var editAmount = document.getElementById("edit-amount");
  var editCategory = document.getElementById("edit-category");
  var editDate = document.getElementById("edit-date");
  var editNote = document.getElementById("edit-note");

  var month = currentMonth();

  function pad(n) { return String(n).padStart(2, "0"); }
  function currentMonth() {
    var d = new Date();
    return d.getFullYear() + "-" + pad(d.getMonth() + 1);
  }
  function euros(cents) { return "€ " + (cents / 100).toFixed(2); }
  function esc(s) {
    return String(s).replace(/[&<>"]/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c];
    });
  }
  function label(key) { return catMap[key] || { icon: "", label: key }; }

  function monthText(m) {
    var parts = m.split("-");
    var d = new Date(Number(parts[0]), Number(parts[1]) - 1, 1);
    return d.toLocaleDateString("en", { month: "long", year: "numeric" });
  }

  function shiftMonth(m, delta) {
    var parts = m.split("-");
    var d = new Date(Number(parts[0]), Number(parts[1]) - 1 + delta, 1);
    return d.getFullYear() + "-" + pad(d.getMonth() + 1);
  }

  function unauthorized(res) {
    if (res.status === 401) { window.location.href = "/login"; return true; }
    return false;
  }

  function load() {
    monthLabelEl.textContent = monthText(month);
    fetch("/api/summary?month=" + month)
      .then(function (r) { if (unauthorized(r)) throw 0; return r.json(); })
      .then(renderSummary)
      .catch(function () {});
    fetch("/api/expenses?month=" + month)
      .then(function (r) { if (unauthorized(r)) throw 0; return r.json(); })
      .then(renderHistory)
      .catch(function () {});
  }

  function renderSummary(sum) {
    totalEl.textContent = euros(sum.total_cents);
    if (!sum.by_category.length) {
      breakdownEl.innerHTML = '<p class="empty">No spending this month.</p>';
      return;
    }
    breakdownEl.innerHTML = sum.by_category.map(function (row) {
      var c = label(row.category_key);
      return (
        '<div class="brow">' +
        '<span class="cat-icon">' + c.icon + "</span>" +
        '<span class="cat-label">' + esc(c.label) + "</span>" +
        '<span class="amt">' + euros(row.total_cents) + "</span></div>"
      );
    }).join("");
  }

  function renderHistory(list) {
    byId = {};
    if (!list.length) {
      historyEl.innerHTML = '<p class="empty">Nothing logged.</p>';
      return;
    }
    historyEl.innerHTML = list.map(function (e) {
      byId[e.id] = e;
      var c = label(e.category_key);
      var note = e.note ? '<span class="hnote">' + esc(e.note) + "</span>" : "";
      return (
        '<button class="hrow" data-id="' + e.id + '">' +
        '<span class="cat-icon">' + c.icon + "</span>" +
        '<span class="hmain"><span class="hcat">' + esc(c.label) + "</span>" + note + "</span>" +
        '<span class="hmeta"><span class="amt">' + euros(e.amount_cents) + "</span>" +
        '<span class="hdate">' + esc(e.spent_on) + "</span></span></button>"
      );
    }).join("");
    Array.prototype.forEach.call(historyEl.querySelectorAll(".hrow"), function (b) {
      b.addEventListener("click", function () { openEditor(Number(b.getAttribute("data-id"))); });
    });
  }

  // --- edit / delete (U7) ---

  function openEditor(id) {
    var e = byId[id];
    if (!e) return;
    editingId = id;
    editAmount.value = (e.amount_cents / 100).toFixed(2);
    editCategory.value = e.category_key;
    editDate.value = e.spent_on;
    editNote.value = e.note || "";
    editBg.hidden = false;
  }
  function closeEditor() {
    editBg.hidden = true;
    editingId = null;
  }
  function toCents(euroStr) {
    var n = Math.round(parseFloat(euroStr) * 100);
    return isFinite(n) ? n : 0;
  }

  document.getElementById("edit-save").addEventListener("click", function () {
    var cents = toCents(editAmount.value);
    if (cents <= 0) { editAmount.focus(); return; }
    fetch("/api/expenses/" + editingId, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        amount_cents: cents,
        category_key: editCategory.value,
        spent_on: editDate.value,
        note: editNote.value.trim(),
      }),
    }).then(function (r) {
      if (unauthorized(r)) return;
      closeEditor();
      load();
    });
  });

  document.getElementById("edit-delete").addEventListener("click", function () {
    if (!window.confirm("Delete this expense?")) return;
    fetch("/api/expenses/" + editingId, { method: "DELETE" }).then(function (r) {
      if (unauthorized(r)) return;
      closeEditor();
      load();
    });
  });

  document.getElementById("edit-cancel").addEventListener("click", closeEditor);
  editBg.addEventListener("click", function (e) {
    if (e.target === editBg) closeEditor();
  });

  // --- month navigation (U6, R18) ---
  document.getElementById("prev").addEventListener("click", function () {
    month = shiftMonth(month, -1);
    load();
  });
  document.getElementById("next").addEventListener("click", function () {
    month = shiftMonth(month, 1);
    load();
  });

  load();
})();
