document.addEventListener("click", (e) => {
  const btn = e.target.closest("[data-explain-start]");
  if (!btn) return;

  const name = btn.getAttribute("data-service-name") || "";
  if (!name) return;

  // вызывай свою функцию
  advncdExplainStart(name);
});


const advncdExplainStart = (name) => {
    const card = document.getElementById("explain-card");
    const pre = card.querySelector("pre");
    const hint = card.querySelector("[data-explain-hint]");
    const spinner = card.querySelector("#explain-loading");
    const btns = document.querySelectorAll("button[onclick*='advncdExplainStart']");
    const statusEl = card.querySelector("#explain-status");
    
    // show spinner
    if (spinner) spinner.style.display = "inline-block";

    // disable buttons
    btns.forEach(b => b.disabled = true);

    // ensure we have a <pre> to write into
    let out = pre;
    if (!out) {
      if (hint) hint.remove();
      out = document.createElement("pre");
      out.className = "whitespace-pre-wrap text-sm";
      card.querySelector(".card-body").appendChild(out);
    }
    out.textContent = "Starting…\n";

    const es = new EventSource(`/services/${encodeURIComponent(name)}/explain/stream`);
    es.onmessage = (ev) => {    
    const msg = JSON.parse(ev.data);

    if (msg.type === "status") {
        // статус НЕ должен затирать накопленный текст
        // либо показывай его отдельным элементом, либо игнорируй после старта
        statusEl.textContent = msg.text;
        return;
    }

    if (msg.type === "delta") {
        out.textContent += msg.text;
        return;
    }

    if (msg.type === "done") {
        // ничего не перезаписываем — только закрываем стрим и UI cleanup
        es.close();
        spinner.style.display = "none";
        btn.disabled = false;
        statusEl.textContent = "";
        return;
    }

    if (msg.type === "error") {
        out.textContent += "\n\nError: " + msg.text;
        es.close();
        spinner.style.display = "none";
        btn.disabled = false;
        return;
    }
    };
    es.onerror = () => {
      out.textContent = "Error: stream connection failed";
      try { es.close(); } catch (_) {}
    };
    es.addEventListener("close", () => {
      // no-op
    });

    const cleanup = () => {
      if (spinner) spinner.style.display = "none";
      btns.forEach(b => b.disabled = false);
    };

    // close handlers call cleanup after small delay
    const originalClose = es.close.bind(es);
    es.close = () => { originalClose(); cleanup(); };
  };