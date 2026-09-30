(function () {
  "use strict";

  const ALLOWED = [".wav", ".mp3", ".m4a", ".flac", ".ogg", ".mp4"];
  let asrDefault = "medium";

  const dropzone = document.getElementById("dropzone");
  const windowbar = document.getElementById("windowbar");
  const btnWindowMinimise = document.getElementById("btn-window-minimise");
  const btnWindowMaximise = document.getElementById("btn-window-maximise");
  const btnWindowClose = document.getElementById("btn-window-close");
  const fileNameEl = document.getElementById("file-name");
  const asrModel = document.getElementById("asr-model");
  const asrLang = document.getElementById("asr-lang");
  const summarySize = document.getElementById("summary-size");
  const customPrompt = document.getElementById("custom-prompt");
  const btnSubmit = document.getElementById("btn-submit");
  const btnReset = document.getElementById("btn-reset");
  const btnStopCurrent = document.getElementById("btn-stop-current");
  const btnStopProgress = document.getElementById("btn-stop-progress");
  const btnRequeueOpen = document.getElementById("btn-requeue-open");
  const btnSummarizeOnlyOpen = document.getElementById("btn-summarize-only-open");
  const formError = document.getElementById("form-error");
  const progressSection = document.getElementById("progress-section");
  const statusLine = document.getElementById("status-line");
  const progressFill = document.getElementById("progress-fill");
  const resultsSection = document.getElementById("results-section");
  const metaLine = document.getElementById("meta-line");
  const outTranscript = document.getElementById("out-transcript");
  const outSummary = document.getElementById("out-summary");
  const speakerFilter = document.getElementById("speaker-filter");
  const speakerFilterWrap = document.getElementById("speaker-filter-wrap");
  const btnCopyTranscript = document.getElementById("btn-copy-transcript");
  const btnCopySummary = document.getElementById("btn-copy-summary");
  const btnDlTranscriptTxt = document.getElementById("btn-dl-transcript-txt");
  const btnDlTranscriptDoc = document.getElementById("btn-dl-transcript-doc");
  const btnDlSummaryTxt = document.getElementById("btn-dl-summary-txt");
  const btnDlSummaryDoc = document.getElementById("btn-dl-summary-doc");
  const historyList = document.getElementById("history-list");
  const historyEmpty = document.getElementById("history-empty");
  const btnRefreshHistory = document.getElementById("btn-refresh-history");
  const historyToolbar = document.getElementById("history-toolbar");
  const historySelectAll = document.getElementById("history-select-all");
  const btnBulkDeleteOpen = document.getElementById("btn-bulk-delete-open");
  const btnStopAll = document.getElementById("btn-stop-all");
  const historyBulkHint = document.getElementById("history-bulk-hint");
  const historyFeedback = document.getElementById("history-feedback");
  const bulkDeleteModal = document.getElementById("bulk-delete-modal");
  const bulkDeleteBackdrop = document.getElementById("bulk-delete-modal-backdrop");
  const bulkDeleteDesc = document.getElementById("bulk-delete-modal-desc");
  const bulkDeleteConfirm = document.getElementById("bulk-delete-confirm");
  const bulkDeleteCancel = document.getElementById("bulk-delete-cancel");
  const requeueModal = document.getElementById("requeue-modal");
  const requeueBackdrop = document.getElementById("requeue-modal-backdrop");
  const requeueHint = document.getElementById("requeue-modal-hint");
  const requeueAsr = document.getElementById("requeue-asr");
  const requeueLang = document.getElementById("requeue-lang");
  const requeueSummary = document.getElementById("requeue-summary");
  const requeueCustom = document.getElementById("requeue-custom");
  const requeueSubmit = document.getElementById("requeue-submit");
  const requeueCancel = document.getElementById("requeue-cancel");
  const summarizeOnlyModal = document.getElementById("summarize-only-modal");
  const summarizeOnlyBackdrop = document.getElementById(
    "summarize-only-modal-backdrop"
  );
  const summarizeOnlyHint = document.getElementById("summarize-only-modal-hint");
  const summarizeOnlySummary = document.getElementById("summarize-only-summary");
  const summarizeOnlyCustom = document.getElementById("summarize-only-custom");
  const summarizeOnlySubmit = document.getElementById("summarize-only-submit");
  const summarizeOnlyCancel = document.getElementById("summarize-only-cancel");
  const btnSettings = document.getElementById("btn-settings");
  const settingsModal = document.getElementById("settings-modal");
  const settingsBackdrop = document.getElementById("settings-modal-backdrop");
  const settingsDataDir = document.getElementById("settings-data-dir");
  const settingsLlamaBin = document.getElementById("settings-llama-bin");
  const settingsLlamaModel = document.getElementById("settings-llama-model");
  const settingsWhisperBin = document.getElementById("settings-whisper-bin");
  const settingsFfmpegBin = document.getElementById("settings-ffmpeg-bin");
  const settingsModelsDir = document.getElementById("settings-models-dir");
  const settingsRuntime = document.getElementById("settings-runtime");
  const settingsPingStatus = document.getElementById("settings-ping-status");
  const settingsPing = document.getElementById("settings-ping");
  const settingsSave = document.getElementById("settings-save");
  const settingsCancel = document.getElementById("settings-cancel");

  const steps = {
    upload: document.querySelector('.step[data-stage="upload"]'),
    asr: document.querySelector('.step[data-stage="asr"]'),
    summarize: document.querySelector('.step[data-stage="summarize"]'),
  };

  let selectedPath = null;
  let pollTimer = null;
  let currentJobId = null;
  let transcriptFetchedForJob = null;
  let fullTranscript = "";
  let requeueTargetJobId = null;
  let summarizeOnlyTargetJobId = null;
  let activeHistoryId = null;
  /** @type {string|null} last GET /jobs/:id status for UI gating */
  let lastKnownJobStatus = null;

  let progressCreepTimer = null;
  /** @type {"asr" | "summ" | null} */
  let progressCreepPhase = null;
  let displayedProgress = 5;

  function extOf(name) {
    const i = name.lastIndexOf(".");
    return i >= 0 ? name.slice(i).toLowerCase() : "";
  }

  function goApp() {
    const app = window.go && window.go.main && window.go.main.App;
    if (!app) {
      const err = new Error("Приложение ещё запускается");
      err.data = { detail: err.message };
      throw err;
    }
    return app;
  }

  async function goCall(name, args) {
    try {
      const fn = goApp()[name];
      if (typeof fn !== "function") {
        throw new Error("Нет метода " + name);
      }
      return await fn.apply(null, args || []);
    } catch (e) {
      const msg = (e && e.message) || String(e);
      const err = new Error(msg);
      err.data = { detail: msg };
      throw err;
    }
  }

  function basename(path) {
    const norm = String(path || "").replace(/\\/g, "/");
    const i = norm.lastIndexOf("/");
    return i >= 0 ? norm.slice(i + 1) : norm;
  }

  function setPath(path) {
    if (!path) {
      selectedPath = null;
      fileNameEl.textContent = "";
      btnSubmit.disabled = true;
      return;
    }
    const name = basename(path);
    const ext = extOf(name);
    if (!ALLOWED.includes(ext)) {
      showError("Допустимы только файлы: " + ALLOWED.join(", "));
      return;
    }
    hideError();
    selectedPath = path;
    fileNameEl.textContent = name;
    btnSubmit.disabled = false;
  }

  async function pickFile() {
    try {
      const path = await goCall("SelectAudioFile", []);
      if (path) setPath(path);
    } catch (e) {
      showError(e.message || String(e));
    }
  }

  function showError(msg) {
    formError.textContent = msg;
    formError.hidden = false;
  }

  function hideError() {
    formError.hidden = true;
    formError.textContent = "";
  }

  function bindWindowControls() {
    const runtime = window.runtime;
    if (!runtime || !runtime.Environment || !windowbar) return;
    runtime.Environment().then(function (environment) {
      if (environment.platform === "windows") {
        windowbar.hidden = false;
        document.body.classList.add("windows");
      }
    }).catch(function () {});

    btnWindowMinimise.addEventListener("click", function () {
      runtime.WindowMinimise();
    });
    btnWindowMaximise.addEventListener("click", function () {
      runtime.WindowToggleMaximise();
    });
    btnWindowClose.addEventListener("click", function () {
      runtime.Quit();
    });
    windowbar.addEventListener("dblclick", function (e) {
      if (!e.target.closest("button")) runtime.WindowToggleMaximise();
    });
  }

  function stageClass(state) {
    if (state === "done") return "step--done";
    if (state === "processing") return "step--processing";
    if (state === "error") return "step--error";
    if (state === "canceled") return "step--error";
    return "step--pending";
  }

  function stopProgressCreep() {
    if (progressCreepTimer) {
      clearInterval(progressCreepTimer);
      progressCreepTimer = null;
    }
    progressCreepPhase = null;
  }

  function setProgressWidth(pct, instant) {
    const p = Math.max(0, Math.min(100, pct));
    displayedProgress = p;
    progressFill.classList.toggle("progress-bar__fill--instant", !!instant);
    progressFill.style.width = p + "%";
    progressFill.parentElement.setAttribute("aria-valuenow", String(Math.round(p)));
    if (instant) {
      requestAnimationFrame(function () {
        progressFill.classList.remove("progress-bar__fill--instant");
      });
    }
  }

  function applyStages(st) {
    for (const key of Object.keys(steps)) {
      const el = steps[key];
      if (!el) continue;
      el.classList.remove(
        "step--pending",
        "step--processing",
        "step--done",
        "step--error"
      );
      el.classList.add(stageClass(st[key] || "pending"));
    }
    const base = progressFromStages(st);
    const u = st.upload || "pending";
    const a = st.asr || "pending";
    const s = st.summarize || "pending";

    if (
      u === "error" ||
      a === "error" ||
      s === "error" ||
      a === "canceled" ||
      s === "canceled" ||
      s === "done"
    ) {
      stopProgressCreep();
      setProgressWidth(base, false);
      return;
    }

    if (a === "processing") {
      if (progressCreepPhase !== "asr") {
        stopProgressCreep();
        progressCreepPhase = "asr";
        displayedProgress = Math.min(Math.max(displayedProgress, 20), 52);
        if (displayedProgress >= 54) displayedProgress = 26;
        progressCreepTimer = setInterval(function () {
          const cap = 54;
          displayedProgress = Math.min(
            displayedProgress +
              Math.max(0.18, (cap - displayedProgress) * 0.07),
            cap
          );
          setProgressWidth(displayedProgress, true);
        }, 400);
      }
      setProgressWidth(displayedProgress, true);
      return;
    }

    if (s === "processing") {
      if (progressCreepPhase !== "summ") {
        stopProgressCreep();
        progressCreepPhase = "summ";
        displayedProgress = Math.max(displayedProgress, Math.max(base - 8, 68));
        progressCreepTimer = setInterval(function () {
          const cap = 94;
          displayedProgress = Math.min(
            displayedProgress +
              Math.max(0.18, (cap - displayedProgress) * 0.06),
            cap
          );
          setProgressWidth(displayedProgress, true);
        }, 400);
      }
      setProgressWidth(displayedProgress, true);
      return;
    }

    stopProgressCreep();
    setProgressWidth(base, false);
  }

  function progressFromStages(st) {
    const u = st.upload || "pending";
    const a = st.asr || "pending";
    const s = st.summarize || "pending";
    if (
      u === "error" ||
      a === "error" ||
      s === "error" ||
      a === "canceled" ||
      s === "canceled"
    )
      return 100;
    if (s === "done") return 100;
    if (s === "processing") return 78;
    if (a === "done") return 55;
    if (a === "processing") return 33;
    if (u === "done") return 12;
    return 5;
  }

  function statusRu(status, stages) {
    if (status === "queued") return "В очереди…";
    if (status === "processing") {
      if (stages.asr === "processing") return "Транскрибация…";
      if (stages.summarize === "processing") return "Саммари…";
      return "Обработка…";
    }
    if (status === "done") return "Готово";
    if (status === "error") return "Ошибка";
    if (status === "canceled") return "Остановлено";
    return status;
  }

  function stopPoll() {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
    stopProgressCreep();
  }

  async function tryLoadTranscriptEarly() {
    if (!currentJobId || transcriptFetchedForJob === currentJobId) return;
    try {
      const tj = await goCall("GetTranscript", [currentJobId]);
      transcriptFetchedForJob = currentJobId;
      setTranscriptText(tj.transcript || "");
      outSummary.textContent = "Ожидание саммари…";
      resultsSection.hidden = false;
      refreshSummarizeOnlyBtn();
    } catch (e) {
      const msg = (e.message || "").toLowerCase();
      if (msg.indexOf("not ready") >= 0) return;
      if (msg.indexOf("no transcript") >= 0 || msg.indexOf("failed") >= 0) {
        showError(e.data ? formatDetail(e.data) : e.message || String(e));
      }
    }
  }

  function speakerIds(text) {
    const ids = [];
    const re = /^Спикер\s+(\d+):/gm;
    let m;
    while ((m = re.exec(text || ""))) {
      if (ids.indexOf(m[1]) < 0) ids.push(m[1]);
    }
    return ids.sort(function (a, b) {
      return Number(a) - Number(b);
    });
  }

  function setTranscriptText(text) {
    fullTranscript = text || "";
    const ids = speakerIds(fullTranscript);
    if (speakerFilterWrap) {
      speakerFilterWrap.hidden = ids.length < 2;
    }
    if (speakerFilter) {
      const prev = speakerFilter.value;
      speakerFilter.innerHTML = "";
      const all = document.createElement("option");
      all.value = "";
      all.textContent = "Все";
      speakerFilter.appendChild(all);
      ids.forEach(function (id) {
        const o = document.createElement("option");
        o.value = id;
        o.textContent = "Спикер " + id;
        speakerFilter.appendChild(o);
      });
      if (prev && ids.indexOf(prev) >= 0) speakerFilter.value = prev;
    }
    renderTranscriptView();
  }

  function renderTranscriptView() {
    const src = fullTranscript || "";
    const want = speakerFilter && speakerFilter.value;
    if (!want) {
      outTranscript.textContent = src;
      return;
    }
    const blocks = src.split(/\n\n+/);
    const keep = [];
    for (let i = 0; i < blocks.length; i++) {
      if (blocks[i].indexOf("Спикер " + want + ":") === 0) keep.push(blocks[i]);
    }
    outTranscript.textContent = keep.join("\n\n");
  }

  function showSavedTranscript(job, summaryPlaceholder) {
    const t = job && job.transcript ? String(job.transcript) : "";
    if (!t.trim()) return false;
    setTranscriptText(t);
    if (summaryPlaceholder != null) {
      outSummary.textContent = summaryPlaceholder;
    }
    resultsSection.hidden = false;
    return true;
  }

  function refreshSummarizeOnlyBtn() {
    const hasTranscript = !!(
      currentJobId &&
      (fullTranscript || outTranscript.textContent || "").trim().length > 0
    );
    const canRetry =
      lastKnownJobStatus === "done" || lastKnownJobStatus === "error";
    btnSummarizeOnlyOpen.hidden = !(hasTranscript && canRetry);
  }

  function isActiveStatus(status) {
    return status === "queued" || status === "processing";
  }

  function refreshStopCurrentBtn() {
    const show = !!(currentJobId && isActiveStatus(lastKnownJobStatus));
    btnStopCurrent.hidden = !show;
    if (btnStopProgress) btnStopProgress.hidden = !show;
  }

  function formatDetail(data) {
    if (data == null) return "";
    if (typeof data.detail === "string") return data.detail;
    if (data.detail && typeof data.detail === "object") {
      return JSON.stringify(data.detail, null, 2);
    }
    return JSON.stringify(data, null, 2);
  }

  function formatUploadLimitError(data) {
    const detail = formatDetail(data);
    const lower = detail.toLowerCase();
    if (/too long|длительн/.test(lower)) {
      return detail || "Аудио слишком длинное";
    }
    if (detail) {
      return detail;
    }
    return "Файл слишком большой";
  }

  async function pollOnce() {
    if (!currentJobId) return;
    try {
      const job = await goCall("GetJob", [currentJobId]);
      lastKnownJobStatus = job.status;
      statusLine.textContent = statusRu(job.status, job.stages);
      applyStages(job.stages || {});
      refreshSummarizeOnlyBtn();
      refreshStopCurrentBtn();

      // As soon as the backend starts returning a transcript for this job,
      // show it in out-transcript instead of waiting for the whole session
      // (ASR + summarization) to complete.
      if (job.status === "processing") {
        if (!showSavedTranscript(job, "Ожидание саммари…")) {
          await tryLoadTranscriptEarly();
        }
      }

      if (job.status === "done") {
        stopPoll();
        btnSubmit.disabled = false;
        transcriptFetchedForJob = null;
        await loadResult();
        refreshHistory();
        refreshSummarizeOnlyBtn();
        return;
      }
      if (job.status === "error") {
        stopPoll();
        btnSubmit.disabled = false;
        transcriptFetchedForJob = null;
        showError(job.error || "Неизвестная ошибка");
        applyStages(job.stages || {});
        showSavedTranscript(job, "");
        refreshSummarizeOnlyBtn();
        refreshHistory();
        return;
      }
      if (job.status === "canceled") {
        stopPoll();
        btnSubmit.disabled = false;
        transcriptFetchedForJob = null;
        statusLine.textContent = "Остановлено";
        applyStages(job.stages || {});
        showSavedTranscript(job, "");
        refreshSummarizeOnlyBtn();
        refreshStopCurrentBtn();
        refreshHistory();
        return;
      }
    } catch (e) {
      stopPoll();
      btnSubmit.disabled = false;
      transcriptFetchedForJob = null;
      lastKnownJobStatus = null;
      refreshSummarizeOnlyBtn();
      refreshStopCurrentBtn();
      showError(e.data ? formatDetail(e.data) : e.message || String(e));
    }
  }

  async function loadResult() {
    try {
      const res = await goCall("GetResult", [currentJobId]);
      setTranscriptText(res.transcript || "");
      outSummary.textContent = res.summary || "";
      const t = res.timings || {};
      const m = res.model_info || {};
      const parts = [];
      if (t.asr_ms != null) parts.push("ASR: " + t.asr_ms + " ms");
      if (t.summarize_ms != null) parts.push("LLM: " + t.summarize_ms + " ms");
      if (m.whisper_model) parts.push("Whisper: " + m.whisper_model);
      if (m.asr_language) parts.push("Язык: " + m.asr_language);
      if (m.llm_model) parts.push("LLM: " + m.llm_model);
      if (m.ollama_model) parts.push("LLM: " + m.ollama_model);
      if (m.summary_mode) parts.push("Режим: " + m.summary_mode);
      metaLine.textContent = parts.join(" · ");
      resultsSection.hidden = false;
      refreshSummarizeOnlyBtn();
    } catch (e) {
      showError(e.data ? formatDetail(e.data) : e.message || String(e));
    }
  }

  function setActiveHistory(id) {
    activeHistoryId = id;
    historyList.querySelectorAll(".history-item").forEach(function (el) {
      el.classList.toggle(
        "history-item--active",
        el.getAttribute("data-job-id") === id
      );
    });
  }

  function getSelectedJobIds() {
    const ids = [];
    historyList.querySelectorAll(".history-cb:checked").forEach(function (cb) {
      ids.push(cb.getAttribute("data-job-id"));
    });
    return ids;
  }

  function updateBulkDeleteButton() {
    const n = getSelectedJobIds().length;
    btnBulkDeleteOpen.disabled = n === 0;
    btnBulkDeleteOpen.textContent =
      n > 0 ? "Удалить выбранное (" + n + ")" : "Удалить выбранное";
  }

  function updateSelectAllCheckbox() {
    const boxes = historyList.querySelectorAll(".history-cb");
    const total = boxes.length;
    if (!total) {
      historySelectAll.checked = false;
      historySelectAll.indeterminate = false;
      return;
    }
    let c = 0;
    boxes.forEach(function (b) {
      if (b.checked) c += 1;
    });
    historySelectAll.indeterminate = false;
    historySelectAll.checked = c === total && total > 0;
  }

  function clearJobViewIfDeleted(deletedSet) {
    if (!currentJobId || !deletedSet.has(currentJobId)) return;
    stopPoll();
    currentJobId = null;
    lastKnownJobStatus = null;
    activeHistoryId = null;
    transcriptFetchedForJob = null;
    hideError();
    progressSection.hidden = true;
    resultsSection.hidden = true;
    setTranscriptText("");
    outSummary.textContent = "";
    metaLine.textContent = "";
    btnReset.hidden = true;
    btnStopCurrent.hidden = true;
    btnRequeueOpen.hidden = true;
    btnSummarizeOnlyOpen.hidden = true;
    btnSubmit.disabled = !selectedPath;
    applyStages({ upload: "pending", asr: "pending", summarize: "pending" });
    progressFill.style.width = "0%";
    historyList.querySelectorAll(".history-item").forEach(function (el) {
      el.classList.remove("history-item--active");
    });
  }

  async function cancelJobById(jobId, options) {
    const opts = options || {};
    const res = await goCall("CancelJob", [jobId]);
    if (res.canceled && jobId === currentJobId) {
      const job = await goCall("GetJob", [jobId]);
      stopPoll();
      lastKnownJobStatus = "canceled";
      statusLine.textContent = "Остановлено";
      applyStages(job.stages || {});
      btnSubmit.disabled = false;
      transcriptFetchedForJob = null;
      refreshSummarizeOnlyBtn();
      refreshStopCurrentBtn();
    }
    if (!opts.quiet) {
      setHistoryFeedback(
        res.canceled ? "Задача остановлена." : "Задача уже не выполняется.",
        res.canceled ? "ok" : "warn"
      );
    }
    await refreshHistory();
    return res;
  }

  async function cancelCurrentJob() {
    if (!currentJobId) return;
    btnStopCurrent.disabled = true;
    if (btnStopProgress) btnStopProgress.disabled = true;
    try {
      await cancelJobById(currentJobId);
    } catch (e) {
      showError(e.data ? formatDetail(e.data) : e.message || String(e));
    } finally {
      btnStopCurrent.disabled = false;
      if (btnStopProgress) btnStopProgress.disabled = false;
      refreshStopCurrentBtn();
    }
  }

  async function cancelAllActiveJobs() {
    btnStopAll.disabled = true;
    try {
      const res = await goCall("CancelActive", []);
      const canceled = new Set(res.canceled || []);
      if (currentJobId && canceled.has(currentJobId)) {
        const job = await goCall("GetJob", [currentJobId]);
        stopPoll();
        lastKnownJobStatus = "canceled";
        statusLine.textContent = "Остановлено";
        applyStages(job.stages || {});
        btnSubmit.disabled = false;
        transcriptFetchedForJob = null;
        refreshSummarizeOnlyBtn();
        refreshStopCurrentBtn();
      }
      setHistoryFeedback(
        canceled.size ? "Остановлено задач: " + canceled.size : "Активных задач нет.",
        canceled.size ? "ok" : "warn"
      );
      await refreshHistory();
    } catch (e) {
      btnStopAll.disabled = false;
      showError(e.data ? formatDetail(e.data) : e.message || String(e));
    }
  }

  function openBulkDeleteModal() {
    const ids = getSelectedJobIds();
    if (!ids.length) return;
    bulkDeleteDesc.textContent =
      "Будет удалено записей: " +
      ids.length +
      ". Аудиофайлы и связанные данные будут удалены без возможности восстановления.";
    bulkDeleteModal.hidden = false;
    bulkDeleteConfirm.focus();
  }

  function closeBulkDeleteModal() {
    bulkDeleteModal.hidden = true;
  }

  async function confirmBulkDelete() {
    const ids = getSelectedJobIds();
    if (!ids.length) {
      closeBulkDeleteModal();
      return;
    }
    bulkDeleteConfirm.disabled = true;
    try {
      const res = await goCall("BulkDelete", [ids]);
      const deleted = new Set(res.deleted || []);
      const skipped = res.skipped || [];
      closeBulkDeleteModal();
      clearJobViewIfDeleted(deleted);
      historySelectAll.checked = false;
      historySelectAll.indeterminate = false;
      if (deleted.size === 0 && skipped.length) {
        setHistoryFeedback(
          "Ничего не удалено: " +
            skipped.length +
            " — задача выполняется или уже отсутствует.",
          "warn"
        );
      } else {
        let msg = "Удалено записей: " + deleted.size;
        if (skipped.length) {
          msg +=
            ". Не удалено: " +
            skipped.length +
            " (выполняется обработка или не найдено).";
        }
        setHistoryFeedback(msg, skipped.length ? "warn" : "ok");
      }
      await refreshHistory();
      updateBulkDeleteButton();
    } catch (e) {
      closeBulkDeleteModal();
      showError(e.data ? formatDetail(e.data) : e.message || String(e));
    } finally {
      bulkDeleteConfirm.disabled = false;
    }
  }

  async function refreshHistory() {
    try {
      const data = await goCall("ListJobs", [40, 0]);
      const jobs = data.jobs || [];
      const keepSelected = new Set(getSelectedJobIds());
      historyList.innerHTML = "";
      if (!jobs.length) {
        historyEmpty.hidden = false;
        historyToolbar.hidden = true;
        historyBulkHint.hidden = true;
        historySelectAll.checked = false;
        historySelectAll.indeterminate = false;
        btnStopAll.disabled = true;
        setHistoryFeedback("", "ok");
        updateBulkDeleteButton();
        return;
      }
      historyEmpty.hidden = true;
      historyToolbar.hidden = false;
      historyBulkHint.hidden = false;
      let activeCount = 0;
      jobs.forEach(function (j) {
        const isActive = isActiveStatus(j.status);
        if (isActive) activeCount += 1;
        const li = document.createElement("li");
        li.className = "history-item";
        li.setAttribute("data-job-id", j.id);
        const row = document.createElement("div");
        row.className = "history-item__row";
        const wrap = document.createElement("span");
        wrap.className = "history-item__checkwrap";
        const cb = document.createElement("input");
        cb.type = "checkbox";
        cb.className = "history-cb";
        cb.setAttribute("data-job-id", j.id);
        const name = j.original_filename || j.id.slice(0, 8);
        const st = (j.stages && j.stages.summarize) || "";
        const line = name + " · " + j.status + (st ? " · " + st : "");
        cb.setAttribute("aria-label", "Выбрать для удаления: " + name);
        if (keepSelected.has(j.id)) cb.checked = true;
        cb.addEventListener("click", function (e) {
          e.stopPropagation();
        });
        cb.addEventListener("change", function () {
          updateSelectAllCheckbox();
          updateBulkDeleteButton();
        });
        const main = document.createElement("button");
        main.type = "button";
        main.className = "history-item__main";
        main.textContent = line;
        main.addEventListener("click", function () {
          selectHistoryJob(j.id);
        });
        wrap.appendChild(cb);
        row.appendChild(wrap);
        row.appendChild(main);
        if (isActive) {
          const stopBtn = document.createElement("button");
          stopBtn.type = "button";
          stopBtn.className = "btn btn--small btn--danger history-item__stop";
          stopBtn.textContent = "Стоп";
          stopBtn.setAttribute("aria-label", "Остановить задачу: " + name);
          stopBtn.addEventListener("click", async function (e) {
            e.stopPropagation();
            stopBtn.disabled = true;
            try {
              await cancelJobById(j.id);
            } catch (err) {
              stopBtn.disabled = false;
              showError(err.data ? formatDetail(err.data) : err.message || String(err));
            }
          });
          row.appendChild(stopBtn);
        }
        li.appendChild(row);
        historyList.appendChild(li);
      });
      btnStopAll.disabled = activeCount === 0;
      if (activeHistoryId) setActiveHistory(activeHistoryId);
      updateSelectAllCheckbox();
      updateBulkDeleteButton();
    } catch {
      /* ignore list errors */
    }
  }

  async function selectHistoryJob(jobId) {
    hideError();
    displayedProgress = 5;
    stopProgressCreep();
    setActiveHistory(jobId);
    currentJobId = jobId;
    transcriptFetchedForJob = null;
    stopPoll();
    progressSection.hidden = false;
    btnReset.hidden = false;
    btnRequeueOpen.hidden = false;
    btnSubmit.disabled = true;
    try {
      const job = await goCall("GetJob", [jobId]);
      lastKnownJobStatus = job.status;
      statusLine.textContent = statusRu(job.status, job.stages);
      applyStages(job.stages || {});
      refreshStopCurrentBtn();
      if (job.status === "done") {
        progressSection.hidden = true;
        await loadResult();
        refreshSummarizeOnlyBtn();
      } else if (job.status === "processing") {
        if (job.stages && job.stages.asr === "done") {
          await tryLoadTranscriptEarly();
        }
        pollTimer = setInterval(pollOnce, 900);
        pollOnce();
        refreshSummarizeOnlyBtn();
      } else if (job.status === "queued") {
        resultsSection.hidden = true;
        pollTimer = setInterval(pollOnce, 900);
        pollOnce();
        refreshSummarizeOnlyBtn();
      } else if (job.status === "error") {
        progressSection.hidden = true;
        showError(job.error || "Ошибка");
        showSavedTranscript(job, "");
        refreshSummarizeOnlyBtn();
      } else if (job.status === "canceled") {
        progressSection.hidden = false;
        statusLine.textContent = "Остановлено";
        if (!showSavedTranscript(job, "")) {
          resultsSection.hidden = true;
        }
        refreshSummarizeOnlyBtn();
      }
    } catch (e) {
      lastKnownJobStatus = null;
      refreshSummarizeOnlyBtn();
      refreshStopCurrentBtn();
      showError(e.data ? formatDetail(e.data) : e.message || String(e));
    }
  }

  function setSelectValue(el, value, fallback) {
    if (!el) return;
    const next = value || fallback;
    if (next && Array.from(el.options).some(function (o) { return o.value === next; })) {
      el.value = next;
    } else if (fallback) {
      el.value = fallback;
    }
  }

  const LANG_STORAGE_KEY = "atm_asr_language";

  function persistLang() {
    try {
      localStorage.setItem(LANG_STORAGE_KEY, asrLang.value);
    } catch {
      /* ignore */
    }
  }

  function restoreLang() {
    try {
      setSelectValue(asrLang, localStorage.getItem(LANG_STORAGE_KEY), "ru");
    } catch {
      /* ignore */
    }
  }

  function openRequeueModal() {
    if (!currentJobId) return;
    requeueTargetJobId = currentJobId;
    requeueHint.textContent = "Задача " + currentJobId;
    setSelectValue(requeueAsr, asrModel.value, asrDefault);
    setSelectValue(requeueLang, asrLang.value, "ru");
    setSelectValue(requeueSummary, summarySize.value, "executive");
    requeueCustom.value = customPrompt.value || "";
    requeueModal.hidden = false;
    goCall("GetJob", [currentJobId]).then(function (job) {
      if (!job || requeueTargetJobId !== currentJobId) return;
      setSelectValue(requeueAsr, job.asr_preset, requeueAsr.value);
      setSelectValue(requeueLang, job.asr_language, requeueLang.value);
      setSelectValue(requeueSummary, job.summary_size, requeueSummary.value);
      if (typeof job.custom_prompt === "string") {
        requeueCustom.value = job.custom_prompt;
      }
    }).catch(function () {
      /* keep form defaults */
    });
  }

  function closeRequeueModal() {
    requeueModal.hidden = true;
    requeueTargetJobId = null;
  }

  function openSummarizeOnlyModal() {
    if (!currentJobId) return;
    summarizeOnlyTargetJobId = currentJobId;
    summarizeOnlyHint.textContent = "Задача " + currentJobId;
    summarizeOnlySummary.value = summarySize.value;
    summarizeOnlyCustom.value = customPrompt.value || "";
    summarizeOnlyModal.hidden = false;
  }

  function closeSummarizeOnlyModal() {
    summarizeOnlyModal.hidden = true;
    summarizeOnlyTargetJobId = null;
  }

  async function submitSummarizeOnly() {
    if (!summarizeOnlyTargetJobId) return;
    try {
      const jid = summarizeOnlyTargetJobId;
      await goCall("SummarizeOnly", [
        jid,
        summarizeOnlySummary.value,
        (summarizeOnlyCustom.value || "").trim(),
      ]);
      closeSummarizeOnlyModal();
      currentJobId = jid;
      activeHistoryId = currentJobId;
      lastKnownJobStatus = "queued";
      refreshStopCurrentBtn();
      progressSection.hidden = false;
      statusLine.textContent = "В очереди…";
      applyStages({ upload: "done", asr: "done", summarize: "pending" });
      outSummary.textContent = "Ожидание саммари…";
      resultsSection.hidden = false;
      stopPoll();
      pollTimer = setInterval(pollOnce, 900);
      pollOnce();
      refreshHistory();
      refreshSummarizeOnlyBtn();
    } catch (e) {
      showError(e.data ? formatDetail(e.data) : e.message || String(e));
    }
  }

  async function submitRequeue() {
    if (!requeueTargetJobId) return;
    try {
      const jid = requeueTargetJobId;
      await goCall("Requeue", [
        jid,
        requeueAsr.value,
        requeueSummary.value,
        (requeueCustom.value || "").trim(),
        requeueLang.value,
      ]);
      closeRequeueModal();
      currentJobId = jid;
      transcriptFetchedForJob = null;
      activeHistoryId = currentJobId;
      lastKnownJobStatus = "queued";
      refreshStopCurrentBtn();
      progressSection.hidden = false;
      statusLine.textContent = "В очереди…";
      applyStages({ upload: "done", asr: "pending", summarize: "pending" });
      resultsSection.hidden = true;
      stopPoll();
      pollTimer = setInterval(pollOnce, 900);
      pollOnce();
      refreshHistory();
    } catch (e) {
      showError(e.data ? formatDetail(e.data) : e.message || String(e));
    }
  }

  async function submitJob() {
    hideError();
    if (!selectedPath) return;
    displayedProgress = 5;
    stopProgressCreep();
    btnSubmit.disabled = true;
    resultsSection.hidden = true;
    setTranscriptText("");
    outSummary.textContent = "";
    progressSection.hidden = false;
    btnReset.hidden = false;
    btnRequeueOpen.hidden = true;
    btnSummarizeOnlyOpen.hidden = true;
    activeHistoryId = null;
    transcriptFetchedForJob = null;
    lastKnownJobStatus = null;
    applyStages({ upload: "done", asr: "pending", summarize: "pending" });
    statusLine.textContent = "Подготовка файла…";

    try {
      const created = await goCall("CreateJob", [
        selectedPath,
        asrModel.value,
        summarySize.value,
        (customPrompt.value || "").trim(),
        asrLang.value,
      ]);
      currentJobId = created.job_id;
      activeHistoryId = currentJobId;
      lastKnownJobStatus = "queued";
      refreshStopCurrentBtn();
      statusLine.textContent = "В очереди…";
      applyStages({ upload: "done", asr: "pending", summarize: "pending" });
      stopPoll();
      pollTimer = setInterval(pollOnce, 900);
      pollOnce();
      refreshHistory();
    } catch (e) {
      btnSubmit.disabled = false;
      progressSection.hidden = true;
      showError(
        e.status === 413
          ? formatUploadLimitError(e.data)
          : e.data
            ? formatDetail(e.data)
            : e.message || String(e)
      );
    }
  }

  function setHistoryFeedback(message, kind) {
    if (!message) {
      historyFeedback.textContent = "";
      historyFeedback.hidden = true;
      historyFeedback.classList.remove("sidebar__feedback--warn");
      return;
    }
    historyFeedback.textContent = message;
    historyFeedback.hidden = false;
    historyFeedback.classList.toggle("sidebar__feedback--warn", kind === "warn");
  }

  function resetUi() {
    stopPoll();
    setHistoryFeedback("", "ok");
    displayedProgress = 5;
    currentJobId = null;
    selectedPath = null;
    transcriptFetchedForJob = null;
    lastKnownJobStatus = null;
    activeHistoryId = null;
    fileNameEl.textContent = "";
    hideError();
    progressSection.hidden = true;
    resultsSection.hidden = true;
    btnReset.hidden = true;
    btnRequeueOpen.hidden = true;
    btnSummarizeOnlyOpen.hidden = true;
    btnSubmit.disabled = true;
    applyStages({ upload: "pending", asr: "pending", summarize: "pending" });
    historyList.querySelectorAll(".history-item").forEach(function (el) {
      el.classList.remove("history-item--active");
    });
  }

  function copyWithExecCommand(text) {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.style.position = "fixed";
    ta.style.left = "-9999px";
    ta.style.top = "0";
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    ta.setSelectionRange(0, text.length);
    let ok = false;
    try {
      ok = document.execCommand("copy");
    } catch {
      ok = false;
    }
    document.body.removeChild(ta);
    return ok;
  }

  async function copyText(text, btn) {
    const prev = btn.textContent;
    if (!(text || "").trim()) {
      showError("Нет текста для копирования");
      return;
    }
    let ok = false;
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
        ok = true;
      }
    } catch {
      ok = false;
    }
    if (!ok) ok = copyWithExecCommand(text);
    if (ok) {
      btn.textContent = "Скопировано";
      setTimeout(function () {
        btn.textContent = prev;
      }, 1600);
    } else {
      showError(
        "Не удалось скопировать. Выделите текст вручную или скачайте .txt."
      );
    }
  }

  function sanitizeBasename(name) {
    return String(name || "export")
      .replace(/[/\\?%*:|"<>]/g, "_")
      .slice(0, 100);
  }

  function exportBasename(kind) {
    const id = currentJobId ? currentJobId.slice(0, 8) : "export";
    return sanitizeBasename(kind + "_" + id);
  }

  async function saveExport(text, basename, format) {
    try {
      await goCall("SaveExport", [basename + "." + format, format, text]);
    } catch (e) {
      showError(e.data ? formatDetail(e.data) : e.message || String(e));
    }
  }

  dropzone.addEventListener("click", function () {
    pickFile();
  });

  dropzone.addEventListener("keydown", function (e) {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      pickFile();
    }
  });

  btnSubmit.addEventListener("click", submitJob);
  btnReset.addEventListener("click", resetUi);
  btnStopCurrent.addEventListener("click", cancelCurrentJob);
  if (btnStopProgress) btnStopProgress.addEventListener("click", cancelCurrentJob);
  btnRequeueOpen.addEventListener("click", openRequeueModal);
  btnSummarizeOnlyOpen.addEventListener("click", openSummarizeOnlyModal);
  requeueCancel.addEventListener("click", closeRequeueModal);
  requeueBackdrop.addEventListener("click", closeRequeueModal);
  requeueSubmit.addEventListener("click", submitRequeue);
  summarizeOnlyCancel.addEventListener("click", closeSummarizeOnlyModal);
  summarizeOnlyBackdrop.addEventListener("click", closeSummarizeOnlyModal);
  summarizeOnlySubmit.addEventListener("click", submitSummarizeOnly);
  btnRefreshHistory.addEventListener("click", refreshHistory);
  btnStopAll.addEventListener("click", cancelAllActiveJobs);

  historySelectAll.addEventListener("change", function () {
    const on = historySelectAll.checked;
    historyList.querySelectorAll(".history-cb").forEach(function (cb) {
      cb.checked = on;
    });
    updateSelectAllCheckbox();
    updateBulkDeleteButton();
  });

  btnBulkDeleteOpen.addEventListener("click", openBulkDeleteModal);
  bulkDeleteCancel.addEventListener("click", closeBulkDeleteModal);
  bulkDeleteBackdrop.addEventListener("click", closeBulkDeleteModal);
  bulkDeleteConfirm.addEventListener("click", function () {
    confirmBulkDelete();
  });

  document.addEventListener("keydown", function (e) {
    if (e.key !== "Escape") return;
    if (!bulkDeleteModal.hidden) closeBulkDeleteModal();
    if (settingsModal && !settingsModal.hidden) closeSettings();
  });

  btnCopyTranscript.addEventListener("click", function () {
    copyText(outTranscript.textContent, btnCopyTranscript);
  });
  btnCopySummary.addEventListener("click", function () {
    copyText(outSummary.textContent, btnCopySummary);
  });

  if (speakerFilter) {
    speakerFilter.addEventListener("change", renderTranscriptView);
  }

  btnDlTranscriptTxt.addEventListener("click", function () {
    const t = fullTranscript || outTranscript.textContent || "";
    if (!t.trim()) {
      showError("Нет транскрипта для сохранения");
      return;
    }
    saveExport(t, exportBasename("transcript"), "txt");
  });
  btnDlTranscriptDoc.addEventListener("click", function () {
    const t = fullTranscript || outTranscript.textContent || "";
    if (!t.trim()) {
      showError("Нет транскрипта для сохранения");
      return;
    }
    saveExport(t, exportBasename("transcript"), "doc");
  });
  btnDlSummaryTxt.addEventListener("click", function () {
    const t = outSummary.textContent || "";
    if (!t.trim()) {
      showError("Нет саммари для сохранения");
      return;
    }
    saveExport(t, exportBasename("summary"), "txt");
  });
  btnDlSummaryDoc.addEventListener("click", function () {
    const t = outSummary.textContent || "";
    if (!t.trim()) {
      showError("Нет саммари для сохранения");
      return;
    }
    saveExport(t, exportBasename("summary"), "doc");
  });

  async function fillSettings() {
    const s = await goCall("GetSettings", []);
    settingsLlamaBin.value = s.llama_bin || "";
    settingsLlamaModel.value = s.llama_model || "";
    settingsWhisperBin.value = s.whisper_bin || "";
    settingsFfmpegBin.value = s.ffmpeg_bin || "";
    settingsModelsDir.value = s.whisper_models_dir || "";
    settingsDataDir.textContent = "Данные: " + (s.data_dir || "");
    const lines = [
      "ffmpeg: " + (s.resolved_ffmpeg || "не найден"),
      "ffprobe: " + (s.resolved_ffprobe || "не найден"),
      "whisper-cli: " + (s.resolved_whisper || "не найден"),
      "llama-server: " + (s.resolved_llama || "не найден"),
      "GGUF: " + (s.resolved_llm_model || "не найден (скачается при первой задаче)"),
    ];
    if (settingsRuntime) settingsRuntime.textContent = lines.join("\n");
    settingsPingStatus.textContent = "";
  }

  async function openSettings() {
    try {
      await fillSettings();
      settingsModal.hidden = false;
    } catch (e) {
      showError(e.message || String(e));
    }
  }

  function closeSettings() {
    settingsModal.hidden = true;
  }

  async function pingOllama() {
    settingsPingStatus.textContent = "Проверка…";
    try {
      const res = await goCall("PingRuntime", []);
      if (res.ok) {
        settingsPingStatus.textContent = "Runtime готов (ffmpeg, whisper, llama-server).";
      } else {
        settingsPingStatus.textContent =
          res.error || "Runtime не готов. Соберите бандл: make dist";
      }
    } catch (e) {
      settingsPingStatus.textContent = e.message || String(e);
    }
  }

  async function saveSettings() {
    try {
      await goCall("SaveSettings", [
        {
          llama_bin: settingsLlamaBin.value,
          llama_model: settingsLlamaModel.value,
          whisper_bin: settingsWhisperBin.value,
          ffmpeg_bin: settingsFfmpegBin.value,
          whisper_models_dir: settingsModelsDir.value,
        },
      ]);
      settingsPingStatus.textContent = "Сохранено.";
    } catch (e) {
      settingsPingStatus.textContent = e.message || String(e);
    }
  }

  function bindNativeDrop() {
    try {
      if (window.runtime && typeof window.runtime.OnFileDrop === "function") {
        window.runtime.OnFileDrop(function (_x, _y, paths) {
          if (paths && paths.length) setPath(paths[0]);
        }, true);
      }
    } catch {
      /* optional */
    }
  }

  if (btnSettings) btnSettings.addEventListener("click", openSettings);
  if (settingsCancel) settingsCancel.addEventListener("click", closeSettings);
  if (settingsBackdrop) settingsBackdrop.addEventListener("click", closeSettings);
  if (settingsPing) settingsPing.addEventListener("click", pingOllama);
  if (settingsSave) settingsSave.addEventListener("click", saveSettings);

  function fillASRSelect(el, cfg) {
    if (!el || !cfg || !cfg.presets) return;
    el.innerHTML = "";
    cfg.presets.forEach(function (p) {
      const o = document.createElement("option");
      o.value = p.id;
      o.textContent = p.label;
      el.appendChild(o);
    });
    setSelectValue(el, cfg.default, asrDefault);
  }

  async function applyASRConfig() {
    try {
      const cfg = await goCall("GetASRConfig", []);
      if (cfg && cfg.default) asrDefault = cfg.default;
      fillASRSelect(asrModel, cfg);
      fillASRSelect(requeueAsr, cfg);
    } catch {
      /* keep HTML fallback */
    }
  }

  asrLang.addEventListener("change", persistLang);
  restoreLang();
  applyASRConfig();

  bindNativeDrop();
  bindWindowControls();
  refreshHistory();
  setTimeout(refreshHistory, 500);
})();
