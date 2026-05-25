(function () {
  document.addEventListener("htmx:responseError", function (event) {
    var detail = event.detail || {};
    var target = detail.target;
    if (!target) {
      return;
    }

    var status = detail.xhr && detail.xhr.status ? " (" + detail.xhr.status + ")" : "";
    var alert = document.createElement("div");
    alert.className =
      "orivis-hx-error rounded-xl px-4 py-3 text-center text-sm font-medium shadow-sm";
    alert.setAttribute("role", "alert");
    alert.textContent = "Unable to refresh this section" + status + ".";

    target.replaceChildren(alert);
  });
})();
