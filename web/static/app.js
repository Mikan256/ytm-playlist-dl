(() => {
  "use strict";

  const $ = (id) => document.getElementById(id);
  const state = {
    config: null,
    jobId: null,
    pollTimer: null,
    info: null,
    busy: false,
    desktop: false,
    desktopInfo: null,
  };

  const STATUS = {
    queued: ["待機中", "idle"],
    running: ["実行中", "running"],
    done: ["完了", "done"],
    partial: ["一部失敗", "partial"],
    error: ["エラー", "error"],
  };

  async function api(path, options = {}) {
    const response = await fetch(path, {
      headers: { "Content-Type": "application/json" },
      ...options,
    });
    let data = {};
    try {
      data = await response.json();
    } catch (_) {
      data = {};
    }
    if (!response.ok || data.ok === false) {
      throw new Error(data.error || `HTTP ${response.status}`);
    }
    return data;
  }

  function toast(message, isError = false) {
    const element = $("toast");
    element.textContent = message;
    element.classList.toggle("error", isError);
    element.classList.remove("hidden");
    window.clearTimeout(toast.timer);
    toast.timer = window.setTimeout(() => {
      element.classList.add("hidden");
    }, 4500);
  }

  function setBadge(text, kind) {
    const badge = $("job-status");
    badge.textContent = text;
    badge.className = `badge ${kind}`;
  }

  function setServerBadge(text, kind) {
    const badge = $("server-status");
    badge.textContent = text;
    badge.className = `badge ${kind}`;
  }

  function setBusy(busy) {
    state.busy = busy;
    for (const id of ["start-button", "dry-run-button", "info-button"]) {
      $(id).disabled = busy;
    }
  }

  function fillSelect(element, values, labels, selected) {
    element.replaceChildren();
    for (const value of values || []) {
      const option = document.createElement("option");
      option.value = value;
      option.textContent = labels && labels[value] ? labels[value] : value;
      if (value === selected) option.selected = true;
      element.append(option);
    }
  }

  function applyConfig(data) {
    state.config = data;
    const cfg = data.config || {};
    fillSelect($("fmt"), data.formats, data.format_labels, cfg.format);
    fillSelect($("template"), data.templates, data.template_labels, cfg.template);
    fillSelect(
      $("normalize_mode"), data.normalize_modes,
      data.normalize_labels, cfg.normalize_mode
    );
    $("template_string").value = cfg.template_string || "";
    $("loudness").value = cfg.loudness ?? -14;
    $("output").value = cfg.output || "./downloads";
    $("jobs").value = cfg.jobs ?? 1;
    $("normalize_jobs").value = cfg.normalize_jobs ?? 1;
    $("concurrent_fragments").value = cfg.concurrent_fragments ?? 1;
    $("audio_quality").value = cfg.audio_quality || "";
    $("keep_going").checked = Boolean(cfg.keep_going);
    $("force_normalize").checked = Boolean(cfg.force_normalize);
    $("embed_metadata").checked = Boolean(cfg.embed_metadata);
    $("embed_thumbnail").checked = Boolean(cfg.embed_thumbnail);
    $("zip").checked = Boolean(cfg.zip);
    $("config-path").textContent = data.config_path || "";
    $("fmt").dispatchEvent(new Event("change"));
  }

  async function loadConfig() {
    try {
      const data = await api("/api/config");
      applyConfig(data);
      setServerBadge("サーバー接続中", "done");
    } catch (error) {
      setServerBadge("接続エラー", "error");
      toast(`設定の読み込みに失敗しました: ${error.message}`, true);
    }
  }

  async function checkDesktop() {
    try {
      const data = await api("/api/desktop/info");
      state.desktop = Boolean(data.desktop);
      state.desktopInfo = data;
      $("desktop-badge").classList.toggle("hidden", !state.desktop);
      $("browse-output").classList.toggle("hidden", !state.desktop);
      $("update-button").classList.toggle(
        "hidden", !state.desktop || !data.updater_ready
      );
    } catch (_) {
      state.desktop = false;
    }
  }

  async function browseOutput() {
    if (!state.desktop) {
      toast("フォルダ選択はデスクトップ版でのみ利用できます", true);
      return;
    }
    try {
      const data = await api("/api/desktop/choose-directory", {
        method: "POST",
        body: JSON.stringify({ current: $("output").value.trim() }),
      });
      if (data.path) {
        $("output").value = data.path;
      }
    } catch (error) {
      toast(`フォルダ選択に失敗しました: ${error.message}`, true);
    }
  }

  async function openDesktopPath(path, reveal = false) {
    if (!state.desktop || !path) return;
    try {
      await api("/api/desktop/open", {
        method: "POST",
        body: JSON.stringify({ path, reveal }),
      });
    } catch (error) {
      toast(`開けませんでした: ${error.message}`, true);
    }
  }

  async function checkForUpdates() {
    if (!state.desktop) return;
    try {
      await api("/api/desktop/update", { method: "POST", body: "{}" });
      toast("更新を確認しています...");
    } catch (error) {
      toast(`更新確認に失敗しました: ${error.message}`, true);
    }
  }

  function optionalInt(id) {
    const raw = $(id).value.trim();
    return raw ? Number(raw) : null;
  }

  function collectPayload() {
    return {
      url: $("url").value.trim(),
      fmt: $("fmt").value,
      template: $("template").value,
      template_string: $("template_string").value.trim() || null,
      loudness: Number($("loudness").value || -14),
      output: $("output").value.trim() || "./downloads",
      normalize_mode: $("normalize_mode").value,
      force_normalize: $("force_normalize").checked,
      start: optionalInt("start"),
      end: optionalInt("end"),
      keep_going: $("keep_going").checked,
      audio_quality: $("audio_quality").value.trim() || null,
      embed_metadata: $("embed_metadata").checked,
      embed_thumbnail: $("embed_thumbnail").checked,
      zip: $("zip").checked,
      jobs: Number($("jobs").value || 1),
      normalize_jobs: Number($("normalize_jobs").value || 1),
      concurrent_fragments: Number($("concurrent_fragments").value || 1),
      dry_run: false,
    };
  }

  function durationLabel(seconds) {
    if (!seconds || Number.isNaN(Number(seconds))) return "";
    const total = Math.round(Number(seconds));
    const minutes = Math.floor(total / 60);
    const rest = String(total % 60).padStart(2, "0");
    return `${minutes}:${rest}`;
  }

  function renderInfo(info) {
    state.info = info;
    $("playlist-title").textContent = info.title || "Unknown";
    $("playlist-count").textContent = `${info.total || 0}曲`;
    const list = $("track-list");
    list.replaceChildren();
    for (const entry of info.entries || []) {
      const row = document.createElement("div");
      row.className = "track";

      const number = document.createElement("span");
      number.className = "num";
      number.textContent = String(entry.index || "").padStart(2, "0");

      const title = document.createElement("span");
      title.textContent = entry.title || "Unknown";

      const duration = document.createElement("span");
      duration.className = "dur";
      duration.textContent = durationLabel(entry.duration);

      row.append(number, title, duration);
      list.append(row);
    }
    $("playlist-card").classList.remove("hidden");
  }

  async function fetchInfo() {
    const payload = collectPayload();
    if (!payload.url) {
      toast("プレイリストURLを入力してください", true);
      $("url").focus();
      return;
    }
    setBusy(true);
    setBadge("情報取得中", "running");
    try {
      const data = await api("/api/info", {
        method: "POST",
        body: JSON.stringify({ url: payload.url }),
      });
      renderInfo(data.info);
      setBadge("情報取得完了", "done");
      toast(`プレイリスト: ${data.info.title}（${data.info.total}曲）`);
    } catch (error) {
      setBadge("エラー", "error");
      toast(`情報取得に失敗しました: ${error.message}`, true);
    } finally {
      setBusy(false);
    }
  }

  function stopPolling() {
    if (state.pollTimer) {
      window.clearTimeout(state.pollTimer);
      state.pollTimer = null;
    }
  }

  async function startJob(dryRun) {
    if (state.busy) return;
    const payload = collectPayload();
    if (!payload.url) {
      toast("プレイリストURLを入力してください", true);
      $("url").focus();
      return;
    }
    payload.dry_run = dryRun;
    if (dryRun) payload.normalize_mode = "off";

    stopPolling();
    state.jobId = null;
    $("log").replaceChildren();
    $("summary").classList.add("hidden");
    $("summary").replaceChildren();
    $("progress-bar").style.width = "0%";
    $("progress-label").textContent = dryRun
      ? "ドライランを実行中..."
      : "ジョブを開始しています...";
    setBadge(dryRun ? "ドライラン" : "開始中", "running");
    setBusy(true);

    try {
      const data = await api("/api/jobs", {
        method: "POST",
        body: JSON.stringify(payload),
      });
      state.jobId = data.job.id;
      pollJob();
    } catch (error) {
      setBusy(false);
      setBadge("エラー", "error");
      $("progress-label").textContent = error.message;
      toast(`開始に失敗しました: ${error.message}`, true);
    }
  }

  function logLine(event) {
    const item = document.createElement("li");
    const time = document.createElement("time");
    const stamp = event.at ? new Date(event.at * 1000) : new Date();
    time.textContent = stamp.toLocaleTimeString("ja-JP", { hour12: false });
    const message = document.createElement("span");

    if (event.type === "download") {
      message.textContent =
        `[${event.index}/${event.total}] ダウンロード: ${event.title}`;
      message.className = "ok";
    } else if (event.type === "normalize") {
      const phase = event.phase === "analysing" ? "解析" : "正規化";
      message.textContent =
        `[${event.current}/${event.total}] ${phase}: ${event.label}`;
    } else if (event.type === "error") {
      message.textContent = event.message || "エラー";
      message.className = "error";
    } else {
      message.textContent = event.message || "";
    }
    item.append(time, message);
    return item;
  }

  function updateProgress(events) {
    let downloadEvent = null;
    let normalizeEvent = null;
    for (const event of events) {
      if (event.type === "download") downloadEvent = event;
      if (event.type === "normalize") normalizeEvent = event;
    }
    const event = normalizeEvent || downloadEvent;
    if (!event) return;
    const current = Number(event.index ?? event.current ?? 0);
    const total = Number(event.total || 0);
    const percent = total > 0 ? Math.min(100, (current / total) * 100) : 0;
    $("progress-bar").style.width = `${percent}%`;
    if (event.type === "download") {
      $("progress-label").textContent =
        `${current}/${total} 曲目をダウンロード: ${event.title}`;
    } else {
      const phase = event.phase === "analysing" ? "解析" : "正規化";
      $("progress-label").textContent =
        `${current}/${total} ${phase}: ${event.label}`;
    }
  }

  function summaryRow(label, value) {
    const row = document.createElement("div");
    const strong = document.createElement("strong");
    strong.textContent = `${label}: `;
    const span = document.createElement("span");
    if (value instanceof Node) {
      span.append(value);
    } else {
      span.textContent = value;
    }
    row.append(strong, span);
    return row;
  }

  function smallButton(label, onClick) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "ghost small";
    button.textContent = label;
    button.addEventListener("click", onClick);
    return button;
  }

  function renderSummary(job) {
    const box = $("summary");
    const result = job.result;
    box.replaceChildren();
    box.classList.remove("hidden", "warn", "error");

    if (job.status === "error") {
      box.classList.add("error");
      box.append(summaryRow("エラー", job.error || "不明なエラー"));
      return;
    }
    if (!result) return;

    if (result.exit_code === 2) box.classList.add("warn");
    if (result.exit_code && result.exit_code !== 2) box.classList.add("error");

    box.append(summaryRow(
      "タイトル", result.info?.title || state.info?.title || "-"
    ));
    box.append(summaryRow("出力先", result.output_dir || "-"));
    if (state.desktop && result.output_dir) {
      box.append(summaryRow(
        "フォルダ",
        smallButton("出力フォルダを開く", () => {
          openDesktopPath(result.output_dir, false);
        })
      ));
    }
    box.append(summaryRow("保存曲数", String((result.files || []).length)));
    if (result.archive_path) {
      const archive = document.createElement("a");
      archive.href = `/api/jobs/${state.jobId}/archive`;
      archive.textContent = "ZIPをダウンロード";
      const archiveWrap = document.createElement("span");
      archiveWrap.append(archive);
      if (state.desktop) {
        archiveWrap.append(" ");
        archiveWrap.append(smallButton("ZIPを開く", () => {
          openDesktopPath(result.archive_path, true);
        }));
      }
      box.append(summaryRow("アーカイブ", archiveWrap));
    }

    if (result.planned && result.planned.length) {
      box.append(summaryRow("予定曲数", String(result.planned.length)));
    }
    if (result.normalization) {
      const normalization = result.normalization;
      box.append(summaryRow(
        "音量処理",
        `${normalization.mode}: 実行 ${normalization.normalized.length} / `
        + `スキップ ${normalization.skipped.length} / `
        + `失敗 ${normalization.failed.length}`
      ));
    }
    const failed = (result.download?.failed || [])
      .concat(result.normalization?.failed || []);
    if (failed.length) {
      box.append(summaryRow("失敗", failed.slice(0, 8).join(" / ")));
    }
    if (state.jobId) {
      const link = document.createElement("a");
      link.href = `/api/jobs/${state.jobId}/report`;
      link.target = "_blank";
      link.rel = "noopener";
      link.textContent = "JSONレポートを開く";
      box.append(summaryRow("レポート", link));
    }
  }

  function renderJob(job) {
    const status = STATUS[job.status] || [job.status, "idle"];
    setBadge(status[0], status[1]);
    const events = job.events || [];
    const log = $("log");
    log.replaceChildren();
    for (const event of events.slice(-300)) {
      log.append(logLine(event));
    }
    log.scrollTop = log.scrollHeight;
    updateProgress(events);
    renderSummary(job);

    if (job.status === "done") {
      $("progress-bar").style.width = "100%";
      $("progress-label").textContent = "完了しました";
      setBusy(false);
      stopPolling();
      toast("処理が完了しました");
    } else if (job.status === "partial") {
      $("progress-bar").style.width = "100%";
      $("progress-label").textContent = "一部の曲でエラーが発生しました";
      setBusy(false);
      stopPolling();
      toast("一部の曲でエラーが発生しました", true);
    } else if (job.status === "error") {
      $("progress-label").textContent = job.error || "エラー";
      setBusy(false);
      stopPolling();
      toast(`エラー: ${job.error || "不明なエラー"}`, true);
    }
  }

  async function pollJob() {
    if (!state.jobId) return;
    try {
      const data = await api(`/api/jobs/${state.jobId}`);
      renderJob(data.job);
      if (["done", "partial", "error"].includes(data.job.status)) return;
    } catch (error) {
      setBadge("エラー", "error");
      $("progress-label").textContent = error.message;
      setBusy(false);
      toast(`状況取得に失敗しました: ${error.message}`, true);
      return;
    }
    state.pollTimer = window.setTimeout(pollJob, 800);
  }

  async function saveConfig() {
    const payload = collectPayload();
    try {
      const data = await api("/api/config", {
        method: "POST",
        body: JSON.stringify(payload),
      });
      toast(`設定を保存しました: ${data.path}`);
    } catch (error) {
      toast(`設定の保存に失敗しました: ${error.message}`, true);
    }
  }

  function bind() {
    $("download-form").addEventListener("submit", (event) => {
      event.preventDefault();
      startJob(false);
    });
    $("info-button").addEventListener("click", fetchInfo);
    $("dry-run-button").addEventListener("click", () => startJob(true));
    $("save-config-button").addEventListener("click", saveConfig);
    $("browse-output").addEventListener("click", browseOutput);
    $("update-button").addEventListener("click", checkForUpdates);
    $("clear-log").addEventListener("click", () => $("log").replaceChildren());
    $("url").addEventListener("keydown", (event) => {
      if (event.key === "Enter") {
        event.preventDefault();
        fetchInfo();
      }
    });
    $("fmt").addEventListener("change", () => {
      const mp3 = $("fmt").value === "mp3";
      $("audio_quality").disabled = !mp3;
      if (!mp3) $("audio_quality").value = "";
    });
  }

  window.addEventListener("DOMContentLoaded", async () => {
    bind();
    await loadConfig();
    await checkDesktop();
  });
})();
