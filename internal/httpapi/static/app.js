const page = document.body.dataset.page;

const riskLabels = {
  high: "高风险",
  medium: "中风险",
  low: "低风险",
};

function $(id) {
  return document.getElementById(id);
}

async function postJSON(url, payload) {
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload || {}),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || `HTTP ${res.status}`);
  }
  return data;
}

async function getJSON(url) {
  const res = await fetch(url);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || `HTTP ${res.status}`);
  }
  return data;
}

function formatTime(value) {
  if (!value) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function joinValues(values) {
  return Array.isArray(values) && values.length ? values.join("、") : "-";
}

function saveUserID(userID) {
  if (userID) localStorage.setItem("bc_agent_user_id", userID);
}

function loadUserID() {
  return localStorage.getItem("bc_agent_user_id") || "demo-user";
}

function track(eventType, payload = {}) {
  return postJSON("/api/events", {
    event_type: eventType,
    assessment_id: payload.assessment_id || "",
    user_id: payload.user_id || loadUserID(),
    metadata: payload.metadata || {},
  }).catch(() => {});
}

function initInputPage() {
  $("user-id").value = loadUserID();
  track("assessment_started", { metadata: { page: "input" } });

  document.querySelectorAll("[data-example]").forEach((button) => {
    button.addEventListener("click", () => {
      $("description").value = button.dataset.example;
      $("description").focus();
    });
  });

  $("assessment-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const status = $("submit-status");
    const userID = $("user-id").value.trim() || "demo-user";
    const description = $("description").value.trim();
    const followUpAnswer = $("followup-answer") ? $("followup-answer").value.trim() : "";
    const followUpAnswers = followUpAnswer ? { user_answer: followUpAnswer } : {};
    saveUserID(userID);

    status.textContent = "评估中...";
    try {
      const response = await postJSON("/api/assessments", {
        user_id: userID,
        description,
        follow_up_answers: followUpAnswers,
      });
      const assessment = response.assessment;
      if (response.needs_follow_up && !followUpAnswer) {
        renderFollowUps(response.follow_up_questions || [], assessment.id);
        status.textContent = "AI 已生成追问，可补充后重新提交。";
        return;
      }
      window.location.href = `/result?id=${encodeURIComponent(assessment.id)}`;
    } catch (err) {
      status.textContent = err.message;
    }
  });
}

function renderFollowUps(questions, assessmentID) {
  const panel = $("followup-panel");
  const list = $("followup-questions");
  list.innerHTML = "";
  questions.forEach((question) => {
    const item = document.createElement("li");
    item.textContent = question;
    list.appendChild(item);
  });
  const link = $("current-result-link");
  link.href = `/result?id=${encodeURIComponent(assessmentID)}`;
  link.classList.remove("hidden");
  panel.classList.remove("hidden");
}

async function initResultPage() {
  const params = new URLSearchParams(window.location.search);
  const id = params.get("id");
  if (!id) {
    $("user-explanation").textContent = "缺少评估 ID。";
    return;
  }

  try {
    const assessment = await getJSON(`/api/assessments/${encodeURIComponent(id)}`);
    renderResult(assessment);
    saveUserID(assessment.user_id);
    track("result_viewed", {
      assessment_id: assessment.id,
      user_id: assessment.user_id,
      metadata: { page: "result" },
    });
  } catch (err) {
    $("user-explanation").textContent = err.message;
  }
}

function renderResult(assessment) {
  const risk = assessment.risk_level;
  const badge = $("risk-badge");
  badge.textContent = riskLabels[risk] || risk;
  badge.className = `risk risk-${risk}`;
  $("generated-at").textContent = formatTime(assessment.generated_at);
  $("contact-hint").textContent = assessment.advice.contact_team ? "建议联系团队" : "暂不强制联系团队";
  $("user-explanation").textContent = assessment.ai_analysis.user_explanation || assessment.evidence.reason;

  const steps = $("next-steps");
  steps.innerHTML = "";
  (assessment.advice.next_steps || []).forEach((step) => {
    const li = document.createElement("li");
    li.textContent = step;
    steps.appendChild(li);
  });

  $("audit-id").textContent = assessment.id;
  $("audit-rule").textContent = `${assessment.evidence.matched_rule_id} / ${assessment.evidence.matched_rule_name}`;
  $("audit-version").textContent = assessment.rule_version;
  $("audit-keywords").textContent = joinValues(assessment.evidence.matched_keywords);
  $("audit-signals").textContent = joinValues(assessment.evidence.ai_signals);
  $("audit-ai").textContent = assessment.ai_analysis.generated_by || "-";
  $("ai-summary").textContent = assessment.ai_analysis.summary || "-";

  $("contact-team").disabled = !assessment.advice.contact_team;
  $("contact-team").addEventListener("click", () => contactTeam(assessment.id));
  $("close-assessment").addEventListener("click", () => closeAssessment(assessment.id));
}

async function contactTeam(id) {
  const status = $("result-status");
  status.textContent = "创建协同请求...";
  try {
    const response = await postJSON(`/api/assessments/${encodeURIComponent(id)}/contact-requests`, {
      channel: "care_team",
      message: "用户在结果页点击联系团队。",
    });
    $("handoff-summary").textContent = response.contact_request.handoff_summary;
    $("handoff-panel").classList.remove("hidden");
    status.textContent = "已创建协同请求。";
  } catch (err) {
    status.textContent = err.message;
  }
}

async function closeAssessment(id) {
  const status = $("result-status");
  status.textContent = "关闭中...";
  try {
    await postJSON(`/api/assessments/${encodeURIComponent(id)}/close`, {});
    status.textContent = "评估已关闭。";
  } catch (err) {
    status.textContent = err.message;
  }
}

function initHistoryPage() {
  $("history-user-id").value = loadUserID();
  $("history-form").addEventListener("submit", (event) => {
    event.preventDefault();
    const userID = $("history-user-id").value.trim() || "demo-user";
    saveUserID(userID);
    loadHistory(userID);
  });
  loadHistory(loadUserID());
}

async function loadHistory(userID) {
  const list = $("history-list");
  list.innerHTML = "<p class=\"muted\">加载中...</p>";
  try {
    const response = await getJSON(`/api/history?user_id=${encodeURIComponent(userID)}`);
    const items = response.assessments || [];
    if (!items.length) {
      list.innerHTML = "<p class=\"muted\">暂无历史记录。</p>";
      return;
    }
    list.innerHTML = "";
    items.forEach((item) => {
      const card = document.createElement("a");
      card.className = "history-item";
      card.href = `/result?id=${encodeURIComponent(item.id)}`;
      card.innerHTML = `
        <span class="risk risk-${item.risk_level}">${riskLabels[item.risk_level] || item.risk_level}</span>
        <strong>${escapeHTML(item.ai_analysis.summary || item.description)}</strong>
        <span>${formatTime(item.generated_at)} · ${escapeHTML(item.evidence.matched_rule_id)}</span>
      `;
      list.appendChild(card);
    });
  } catch (err) {
    list.innerHTML = `<p class="muted">${escapeHTML(err.message)}</p>`;
  }
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

if (page === "input") initInputPage();
if (page === "result") initResultPage();
if (page === "history") initHistoryPage();
