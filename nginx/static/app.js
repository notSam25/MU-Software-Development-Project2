//Persisted keys for frontend session cache
const STORAGE_KEY = "group9_iPERMITAPP_state_v2";
const INITIAL_STATE = {
  auth: null,
  trackedRequests: [],
  reProfiles: {},
};

//Backend account types by ui role
const ACCOUNT_TYPES = {
  re: "regulated_entity",
  eo: "environmental_officer",
};

//Ui role lookup from backend account types
const ACCOUNT_TYPE_TO_UI_ROLE = {
  regulated_entity: "re",
  environmental_officer: "eo",
};

//Display labels for each ui role
const ROLE_LABELS = {
  re: "Regulated Entity",
  eo: "Environmental Officer",
};

//Shared workflow statuses for tracked requests
const STATUS = {
  pendingPayment: "Pending Payment",
  reviewingPayment: "Reviewing Payment",
  submitted: "Submitted",
  rejected: "Rejected",
  beingReviewed: "Being Reviewed",
  accepted: "Accepted",
};

//Tab ids mapped to their initializer functions
const TAB_INITIALIZERS = {
  "re-account": initReAccountTab,
  "re-permit": initRePermitTab,
  "re-payment": initRePaymentTab,
  "re-ack": initReAckTab,
  "eo-account": initEoAccountTab,
  "eo-review": initEoReviewTab,
  "eo-issue": initEoIssueTab,
  "eo-report": initEoReportTab,
};

let state = loadState();
let activeView = "";
let noticeTimer = null;
let environmentalPermits = [];

//Small dom and formatting helpers
const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];
const toFormData = (form) => Object.fromEntries(new FormData(form).entries());
const money = (value) => `$${Number(value || 0).toFixed(2)}`;

init();

//Bootstraps handlers and restores session state
function init() {
  bindAuthForms();
  bindLoginRoleBehavior();
  bindSessionControls();
  bindTabControls();
  renderSessionState();
  refreshSessionFromWhoAmI();
}

function normalizeEmail(value) {
  return String(value || "")
    .trim()
    .toLowerCase();
}

function isSupportedAccountType(accountType) {
  return Boolean(ACCOUNT_TYPE_TO_UI_ROLE[accountType]);
}

//Normalize tracked request fields before storage
function normalizeTrackedItem(item) {
  return {
    id: Number(item?.id || 0),
    ownerEmail: normalizeEmail(item?.ownerEmail),
    activityDescription: String(item?.activityDescription || ""),
    environmentalPermitId: Number(item?.environmentalPermitId || 0),
    permitFee: Number(item?.permitFee || 0),
    status: String(item?.status || ""),
    permitCreated: Boolean(item?.permitCreated),
    updatedAt: item?.updatedAt || "",
    notes: Array.isArray(item?.notes) ? item.notes.slice(-10) : [],
  };
}

//Restore state from local storage and drop invalid records
function loadState() {
  try {
    const parsed = JSON.parse(localStorage.getItem(STORAGE_KEY) || "null");
    const tracked = Array.isArray(parsed?.trackedRequests)
      ? parsed.trackedRequests
          .map(normalizeTrackedItem)
          .filter((item) => item.id > 0)
      : [];

    return {
      ...INITIAL_STATE,
      ...(parsed || {}),
      auth: parsed?.auth?.token ? parsed.auth : null,
      trackedRequests: tracked,
      reProfiles:
        typeof parsed?.reProfiles === "object" && parsed?.reProfiles
          ? parsed.reProfiles
          : {},
    };
  } catch {
    return { ...INITIAL_STATE };
  }
}

function saveState() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
}

//Wire register and login form handlers
function bindAuthForms() {
  $("#register-form")?.addEventListener("submit", onRegister);
  $("#login-form")?.addEventListener("submit", onLogin);
}

//Keep role specific login label and placeholder in sync
function bindLoginRoleBehavior() {
  const roleSelect = $("#login-role");
  const emailInput = $("#login-email");
  const label = $("#login-email-label");
  if (!roleSelect || !emailInput || !label) return;

  const rolePlaceholders = {
    re: "regulated entity email",
    eo: "officer@example.com",
  };
  const textNode = [...label.childNodes].find(
    (node) => node.nodeType === Node.TEXT_NODE,
  );

  const setRoleMode = () => {
    const role = roleSelect.value;
    const roleLabel = ROLE_LABELS[role] || "Account";
    if (textNode) textNode.nodeValue = `Email (${roleLabel}) `;
    emailInput.placeholder = rolePlaceholders[role] || "email";
  };

  roleSelect.addEventListener("change", setRoleMode);
  setRoleMode();
}

function bindSessionControls() {
  $("#logout-btn")?.addEventListener("click", () => {
    clearAuth();
    renderSessionState();
    showNotice("Session ended.", "info");
  });
}

//Wire tab clicks and htmx fragment lifecycle
function bindTabControls() {
  $("#tab-controls")?.addEventListener("click", (event) => {
    const button = event.target.closest(".tab-btn");
    if (!button) return;
    activeView = button.dataset.view || "";
    updateTabStyling(button);
  });

  document.body.addEventListener("htmx:afterSwap", (event) => {
    if (event.detail.target.id === "tab-content") {
      TAB_INITIALIZERS[activeView]?.();
    }
  });
}

async function onRegister(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const data = toFormData(form);

  try {
    //Create account through backend then cache profile fields
    await apiRequest("/register", {
      method: "POST",
      body: {
        contact_person_name: data.contact_person_name,
        password: data.password,
        email: normalizeEmail(data.email),
        organization_name: data.organization_name,
        organization_address: data.organization_address,
      },
      skipAuth: true,
    });

    upsertReProfile({
      email: normalizeEmail(data.email),
      contact_person_name: data.contact_person_name,
      organization_name: data.organization_name,
      organization_address: data.organization_address,
    });

    form.reset();
    showNotice("Regulated entity account created.", "success");
  } catch (error) {
    showNotice(`Registration failed: ${error.message}`, "error");
  }
}

async function onLogin(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const data = toFormData(form);
  const accountType = ACCOUNT_TYPES[data.role];

  if (!accountType) {
    showNotice("Select a valid role.", "error");
    return;
  }

  try {
    //Authenticate then refresh claims from whoami
    const loginResponse = await apiRequest("/login", {
      method: "POST",
      body: {
        account_type: accountType,
        email: String(data.email || "").toLowerCase(),
        password: data.password,
      },
      skipAuth: true,
    });

    state.auth = {
      token: loginResponse.token,
      accountType,
      accountId: 0,
      email: normalizeEmail(data.email),
    };

    await syncSessionFromWhoAmI();

    renderSessionState();
    form.reset();
    showNotice(
      `${ROLE_LABELS[ACCOUNT_TYPE_TO_UI_ROLE[state.auth.accountType]]} signed in.`,
      "success",
    );
  } catch (error) {
    clearAuth();
    renderSessionState();
    showNotice(`Login failed: ${error.message}`, "error");
  }
}

function hideAppPanel(panel, content) {
  //Hide panel and reset tab state
  panel.classList.add("hidden");
  content.innerHTML = "";
  activeView = "";
  updateTabStyling(null);
}

function renderSessionState() {
  const panel = $("#app-panel");
  const sessionTitle = $("#session-title");
  const content = $("#tab-content");
  if (!panel || !sessionTitle || !content) return;

  if (!state.auth?.token) {
    hideAppPanel(panel, content);
    return;
  }

  const uiRole = ACCOUNT_TYPE_TO_UI_ROLE[state.auth.accountType];
  if (!uiRole) {
    clearAuth();
    hideAppPanel(panel, content);
    return;
  }

  const roleLabel = ROLE_LABELS[uiRole] || "Account";

  panel.classList.remove("hidden");
  sessionTitle.textContent = `Active Session: ${state.auth.email} (${roleLabel})`;

  //Render role scoped tabs for active session
  const tabs = $$(".tab-btn");
  tabs.forEach((button) => {
    button.classList.toggle("hidden", button.dataset.role !== uiRole);
  });

  const availableTabs = tabs.filter(
    (button) => !button.classList.contains("hidden"),
  );
  const activeButton =
    availableTabs.find((button) => button.dataset.view === activeView) ||
    availableTabs[0];
  activeButton?.click();
}

function updateTabStyling(activeButton) {
  $$(".tab-btn").forEach((button) => {
    const active = activeButton && button === activeButton;
    button.classList.toggle("bg-gov-600", active);
    button.classList.toggle("text-white", active);
    button.classList.toggle("border-gov-600", active);
    button.classList.toggle("font-semibold", active);
  });
}

async function refreshSessionFromWhoAmI() {
  if (!state.auth?.token) return;

  try {
    await syncSessionFromWhoAmI();
    renderSessionState();
  } catch {
    clearAuth();
    renderSessionState();
  }
}

//Refresh session claims from backend token context
async function syncSessionFromWhoAmI() {
  const who = await apiRequest("/whoami");
  if (!isSupportedAccountType(who.account_type)) {
    throw new Error(
      "This UI currently supports regulated entity and environmental officer workflows only.",
    );
  }

  state.auth = {
    ...state.auth,
    accountType: who.account_type,
    accountId: Number(who.account_id || 0),
    email: normalizeEmail(who.email || state.auth?.email),
  };
  saveState();
  return state.auth;
}

function initReAccountTab() {
  const profile = $("#re-profile");
  const note = $("#re-account-note");
  if (!profile) return;

  const cached = state.reProfiles[state.auth?.email || ""] || {};
  const rows = [
    ["Email", state.auth?.email || ""],
    ["Account Type", "regulated_entity"],
    ["Contact", cached.contact_person_name || "Not available from API"],
    ["Organization", cached.organization_name || "Not available from API"],
    ["Address", cached.organization_address || "Not available from API"],
  ];

  profile.innerHTML = rows
    .map(
      ([label, value]) =>
        `<div><dt class="text-slate-500">${label}</dt><dd>${escapeHtml(value)}</dd></div>`,
    )
    .join("");

  if (note) {
    note.textContent =
      "Password updates are not exposed by the current backend API.";
  }
}

function initRePermitTab() {
  //Load permit templates and submit permit requests
  void loadEnvironmentalPermits().then(() => {
    renderPermitTemplates();
  });
  renderRePermitList();

  bindSubmit("#permit-app-form", async (data, form) => {
    const permitId = Number(
      data.environmental_permit_id_manual || data.environmental_permit_id,
    );
    const durationHours = Number(data.activity_duration_hours);
    const startDateIso = toDateOnlyISO(data.activity_start_date);

    if (!permitId) {
      showNotice("Provide a valid environmental permit ID.", "error");
      return;
    }
    if (!startDateIso) {
      showNotice("Provide a valid activity start date.", "error");
      return;
    }
    if (!Number.isFinite(durationHours) || durationHours <= 0) {
      showNotice(
        "Activity duration must be a positive number of hours.",
        "error",
      );
      return;
    }

    const response = await apiRequest("/request-permit", {
      method: "POST",
      body: {
        activity_description: data.activity_description,
        activity_start_date: startDateIso,
        //Convert hours to nanoseconds for backend duration format
        activity_duration: Math.round(durationHours * 60 * 60 * 1000000000),
        environmental_permit_id: permitId,
      },
    });

    const requestId = Number(response.id || 0);
    if (!requestId) {
      showNotice(
        "Permit request was created but the response was missing an ID.",
        "error",
      );
      return;
    }

    //Track new request locally for downstream tabs
    upsertTrackedRequest({
      id: requestId,
      ownerEmail: state.auth?.email || "",
      activityDescription: data.activity_description,
      environmentalPermitId: permitId,
      permitFee: Number(response.permit_fee || 0),
      status: STATUS.pendingPayment,
      permitCreated: false,
      updatedAt: new Date().toISOString(),
    });
    appendTrackedNote(requestId, "Permit request created.");
    saveState();

    form.reset();
    renderPermitTemplates();
    renderRePermitList();
    showNotice(`Permit request #${requestId} created.`, "success");
  });
}

async function loadEnvironmentalPermits() {
  try {
    const payload = await apiRequest("/environmental-permits", {
      skipAuth: true,
    });
    environmentalPermits = Array.isArray(payload.items)
      ? payload.items
          .map((item) => ({
            id: Number(item?.id || 0),
            permit_name: String(item?.permit_name || ""),
            permit_fee: Number(item?.permit_fee || 0),
            description: String(item?.description || ""),
          }))
          .filter((item) => item.id > 0)
      : [];
  } catch {
    environmentalPermits = [];
  }
}

function initRePaymentTab() {
  //Submit payment for pending permit request
  renderRePaymentSelectors();
  renderRePaymentList();

  bindSubmit("#payment-form", async (data, form) => {
    const requestId = Number(data.request_id_manual || data.request_id);
    if (!requestId) {
      showNotice("Select or enter a permit request ID.", "error");
      return;
    }

    if (!/^\d{4}$/.test(String(data.last_four_digits_of_card || ""))) {
      showNotice(
        "Last four card digits must be exactly four numbers.",
        "error",
      );
      return;
    }

    const response = await apiRequest(
      `/permit-request/${requestId}/submit_payment`,
      {
        method: "POST",
        body: {
          payment_method: data.payment_method,
          last_four_digits_of_card: data.last_four_digits_of_card,
          card_holder_name: data.card_holder_name,
        },
      },
    );

    ensureTrackedRequest(requestId, state.auth?.email || "");
    updateTrackedStatus(
      requestId,
      response.status || STATUS.reviewingPayment,
      "Payment submitted.",
    );
    saveState();

    form.reset();
    renderRePaymentSelectors();
    renderRePaymentList();
    showNotice(`Payment submitted for request #${requestId}.`, "success");
  });
}

function initReAckTab() {
  const rows = trackedRequestsForCurrentRe();
  renderList(
    "#re-ack-list",
    rows,
    "No tracked workflow updates are available.",
    (item) =>
      card(item.id, [
        detail("Current status", item.status || "Unknown"),
        detail("Latest note", latestNote(item) || "No note available"),
      ]),
  );
}

function initEoAccountTab() {
  const profile = $("#eo-profile");
  const note = $("#eo-account-note");
  if (!profile) return;

  profile.innerHTML = `
    <div><dt class="text-slate-500">Email</dt><dd>${escapeHtml(state.auth?.email || "")}</dd></div>
    <div><dt class="text-slate-500">Account Type</dt><dd>environmental_officer</dd></div>
  `;

  if (note) {
    note.textContent =
      "Password updates are not exposed by the current backend API.";
  }
}

async function initEoReviewTab() {
  //Load submitted queue then move requests into review
  await refreshEoSubmittedQueue();
  renderKnownBeingReviewed();

  bindSubmit("#eo-start-review-form", async (data, form) => {
    const requestId = Number(data.request_id);
    if (!requestId) {
      showNotice("Select a submitted permit request.", "error");
      return;
    }

    const response = await apiRequest(
      `/eo/permit-request/${requestId}/start-review`,
      { method: "POST" },
    );
    updateTrackedStatus(
      requestId,
      response.status || STATUS.beingReviewed,
      "EO started review.",
    );
    saveState();

    form.reset();
    await refreshEoSubmittedQueue();
    renderKnownBeingReviewed();
    showNotice(`EO started review for request #${requestId}.`, "success");
  });
}

function initEoIssueTab() {
  //Issue accepted or rejected decision for reviewed requests
  renderKnownBeingReviewed();
  renderEoDecisionSelector();

  bindSubmit("#eo-final-decision-form", async (data, form) => {
    const requestId = Number(data.request_id_manual || data.request_id);
    if (!requestId) {
      showNotice("Select or enter a permit request ID.", "error");
      return;
    }

    const response = await apiRequest("/review-permit", {
      method: "POST",
      body: {
        permit_request_id: requestId,
        decision: data.decision,
        description: data.description,
      },
    });

    const decision = response.decision || data.decision;
    const tracked = findTrackedRequest(requestId);
    if (tracked) tracked.permitCreated = decision === STATUS.accepted;
    updateTrackedStatus(
      requestId,
      decision,
      `EO final decision: ${data.description}`,
    );
    saveState();

    form.reset();
    renderKnownBeingReviewed();
    renderEoDecisionSelector();
    showNotice(
      `EO final decision submitted for request #${requestId}.`,
      "success",
    );
  });
}

async function initEoReportTab() {
  //Build eo summary from tracked requests and live queue count
  let submittedQueueCount = "n/a";
  try {
    const payload = await apiRequest("/eo/permit-requests/submitted-payment");
    const items = Array.isArray(payload.items) ? payload.items : [];
    syncTrackedFromApiItems(items);
    submittedQueueCount = String(items.length);
    saveState();
  } catch {
    submittedQueueCount = "unavailable";
  }

  const rows = sortedTrackedRequests();
  const summaryItems = [
    ["Tracked Requests", rows.length],
    ["Pending Payment", countByStatus(rows, STATUS.pendingPayment)],
    ["Reviewing Payment", countByStatus(rows, STATUS.reviewingPayment)],
    ["Submitted", countByStatus(rows, STATUS.submitted)],
    ["Being Reviewed", countByStatus(rows, STATUS.beingReviewed)],
    ["Accepted", countByStatus(rows, STATUS.accepted)],
    ["Rejected", countByStatus(rows, STATUS.rejected)],
    ["Live Submitted Queue", submittedQueueCount],
  ];

  const summary = $("#eo-report-summary");
  if (summary) {
    summary.innerHTML = summaryItems
      .map(
        ([label, value]) => `
          <div class="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2">
            <p class="font-mono text-xs uppercase tracking-wide text-slate-500">${escapeHtml(label)}</p>
            <p class="text-xl font-semibold text-slate-900">${escapeHtml(value)}</p>
          </div>
        `,
      )
      .join("");
  }

  const table = $("#eo-report-table");
  if (!table) return;

  if (!rows.length) {
    table.innerHTML =
      '<tr><td class="px-2 py-2 text-slate-600" colspan="7">No tracked requests are available.</td></tr>';
    return;
  }

  table.innerHTML = rows
    .map(
      (item) => `
        <tr class="text-slate-800">
          <td class="px-2 py-2 font-mono text-xs">${escapeHtml(item.id)}</td>
          <td class="px-2 py-2">${escapeHtml(item.ownerEmail || "unknown")}</td>
          <td class="px-2 py-2">${escapeHtml(item.activityDescription || "n/a")}</td>
          <td class="px-2 py-2">${escapeHtml(item.environmentalPermitId || "n/a")}</td>
          <td class="px-2 py-2">${escapeHtml(item.status || "Unknown")}</td>
          <td class="px-2 py-2">${escapeHtml(item.permitCreated ? "Created" : "Not created")}</td>
          <td class="px-2 py-2">${escapeHtml(item.updatedAt ? new Date(item.updatedAt).toLocaleString() : "n/a")}</td>
        </tr>
      `,
    )
    .join("");
}

async function refreshEoSubmittedQueue() {
  //Pull submitted queue and sync tracked request records
  try {
    const payload = await apiRequest("/eo/permit-requests/submitted-payment");
    const items = Array.isArray(payload.items) ? payload.items : [];
    syncTrackedFromApiItems(items);
    saveState();

    renderList(
      "#eo-review-list",
      items,
      "No requests are currently in Submitted status.",
      (item) => {
        const id = requestIdFromApi(item);
        const status = latestStatusFromApi(item) || STATUS.submitted;
        return card(id, [
          detail("Status", status),
          detail("Regulated Entity ID", item.RegulatedEntityID || "n/a"),
        ]);
      },
    );

    setSelectOptions(
      "#eo-start-review-form select[name='request_id']",
      "Select submitted request",
      items,
      (item) => requestIdFromApi(item),
      (item) =>
        `#${requestIdFromApi(item)} - ${latestStatusFromApi(item) || STATUS.submitted}`,
    );
  } catch (error) {
    renderList(
      "#eo-review-list",
      [],
      "Unable to load EO submitted queue.",
      () => "",
    );
    showNotice(`Failed to load EO queue: ${error.message}`, "error");
  }
}

function renderPermitTemplates() {
  const permits = environmentalPermits;
  setSelectOptions(
    "#permit-app-form select[name='environmental_permit_id']",
    "Select permit template",
    permits,
    (item) => item.id,
    (item) => `${item.id} - ${item.permit_name} (${money(item.permit_fee)})`,
  );

  const list = $("#permit-template-list");
  if (!list) return;

  if (!permits.length) {
    list.innerHTML =
      '<li class="rounded border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-700">No permit templates are available.</li>';
    return;
  }

  list.innerHTML = permits
    .map(
      (item) => `
        <li class="rounded border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-700">
          <p class="font-semibold text-slate-800">${escapeHtml(item.id)} - ${escapeHtml(item.permit_name)}</p>
          <p>Fee: ${escapeHtml(money(item.permit_fee))}</p>
          ${item.description ? `<p class="mt-1 text-slate-600">${escapeHtml(item.description)}</p>` : ""}
        </li>
      `,
    )
    .join("");
}

function renderRePermitList() {
  const rows = trackedRequestsForCurrentRe();
  renderList(
    "#re-application-list",
    rows,
    "No tracked permit requests for this account.",
    (item) =>
      card(item.id, [
        detail("Status", item.status || "Unknown"),
        detail("Permit Template ID", item.environmentalPermitId || "n/a"),
        detail("Permit Fee", money(item.permitFee || 0)),
        detail("Activity", item.activityDescription || "n/a"),
      ]),
  );
}

function renderRePaymentSelectors() {
  const pending = trackedRequestsForCurrentRe().filter(
    (item) => item.status === STATUS.pendingPayment,
  );
  setSelectOptions(
    "#payment-form select[name='request_id']",
    "Select pending request",
    pending,
    (item) => item.id,
    (item) => `#${item.id} - ${money(item.permitFee || 0)}`,
  );
}

function renderRePaymentList() {
  const rows = trackedRequestsForCurrentRe();
  renderList(
    "#re-payment-list",
    rows,
    "No tracked requests are available for payment.",
    (item) =>
      card(item.id, [
        detail("Status", item.status || "Unknown"),
        detail("Latest note", latestNote(item) || "No note available"),
      ]),
  );
}

function renderKnownBeingReviewed() {
  const rows = sortedTrackedRequests().filter(
    (item) => item.status === STATUS.beingReviewed,
  );
  renderList(
    "#eo-being-reviewed-list",
    rows,
    "No tracked requests are currently in Being Reviewed.",
    (item) =>
      card(item.id, [
        detail("Owner", item.ownerEmail || "unknown"),
        detail("Activity", item.activityDescription || "n/a"),
      ]),
  );
}

function renderEoDecisionSelector() {
  const rows = sortedTrackedRequests().filter(
    (item) => item.status === STATUS.beingReviewed,
  );
  setSelectOptions(
    "#eo-final-decision-form select[name='request_id']",
    "Select being-reviewed request",
    rows,
    (item) => item.id,
    (item) => `#${item.id} - ${item.activityDescription || "request"}`,
  );
}

function bindSubmit(selector, handler) {
  //Wrap form submits with shared error handling
  const form = $(selector);
  if (!form) return;

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    try {
      await handler(toFormData(form), form, event);
    } catch (error) {
      showNotice(error.message || "Request failed.", "error");
    }
  });
}

function renderList(selector, items, emptyText, itemToHtml) {
  //Render helper for list style sections
  const container = $(selector);
  if (!container) return;

  container.innerHTML = items.length
    ? items.map(itemToHtml).join("")
    : `<p class="text-sm text-slate-600">${escapeHtml(emptyText)}</p>`;
}

function setSelectOptions(
  selector,
  placeholder,
  items,
  valueBuilder = (item) => item.id,
  labelBuilder = (item) => item.id,
) {
  //Populate select options from item collection
  const select = $(selector);
  if (!select) return;

  const options = items
    .map((item) => {
      const value = valueBuilder(item);
      const label = labelBuilder(item);
      return `<option value="${escapeHtml(value)}">${escapeHtml(label)}</option>`;
    })
    .join("");

  select.innerHTML = `<option value="">${escapeHtml(placeholder)}</option>${options}`;
}

function card(id, lines, footer = "") {
  return `
    <article class="rounded-lg border border-slate-200 bg-slate-50 p-3 text-sm text-slate-800">
      <p class="font-mono text-xs text-slate-500">${escapeHtml(id)}</p>
      ${lines.join("")}
      ${footer}
    </article>
  `;
}

function detail(label, value) {
  return `<p><span class="text-slate-600">${escapeHtml(label)}:</span> ${escapeHtml(value)}</p>`;
}

//Cache profile fields from registration data
function upsertReProfile(profile) {
  const email = normalizeEmail(profile.email);
  if (!email) return;

  state.reProfiles[email] = {
    email,
    contact_person_name: profile.contact_person_name || "",
    organization_name: profile.organization_name || "",
    organization_address: profile.organization_address || "",
  };
  saveState();
}

function trackedRequestsForCurrentRe() {
  //Track requests for signed in regulated entity
  const email = normalizeEmail(state.auth?.email);
  return sortedTrackedRequests().filter((item) => item.ownerEmail === email);
}

function sortedTrackedRequests() {
  return [...state.trackedRequests].sort((a, b) => Number(b.id) - Number(a.id));
}

function findTrackedRequest(id) {
  //Get tracked request by id
  return state.trackedRequests.find((item) => Number(item.id) === Number(id));
}

function ensureTrackedRequest(id, ownerEmail = "") {
  //Create tracked request shell when missing
  const numericId = Number(id || 0);
  if (!numericId) return null;

  const existing = findTrackedRequest(id);
  if (existing) return existing;

  const created = {
    id: numericId,
    ownerEmail: normalizeEmail(ownerEmail),
    activityDescription: "",
    environmentalPermitId: 0,
    permitFee: 0,
    status: "",
    permitCreated: false,
    updatedAt: new Date().toISOString(),
    notes: [],
  };

  state.trackedRequests.push(created);
  return created;
}

function normalizeTrackedPatch(patch, fallback = {}) {
  //Normalize partial tracked updates before merge
  const permitFee = Number(patch?.permitFee);
  const environmentPermitId = patch?.environmentalPermitId;

  return {
    id: Number(patch?.id || fallback.id || 0),
    ownerEmail:
      patch?.ownerEmail !== undefined
        ? normalizeEmail(patch.ownerEmail)
        : normalizeEmail(fallback.ownerEmail),
    activityDescription:
      patch?.activityDescription ?? fallback.activityDescription ?? "",
    environmentalPermitId:
      environmentPermitId !== undefined &&
      environmentPermitId !== null &&
      environmentPermitId !== ""
        ? Number(environmentPermitId)
        : Number(fallback.environmentalPermitId || 0),
    permitFee: Number.isFinite(permitFee)
      ? permitFee
      : Number(fallback.permitFee || 0),
    status: patch?.status ?? fallback.status ?? "",
    permitCreated:
      typeof patch?.permitCreated === "boolean"
        ? patch.permitCreated
        : Boolean(fallback.permitCreated),
    updatedAt:
      patch?.updatedAt || fallback.updatedAt || new Date().toISOString(),
    notes: Array.isArray(patch?.notes)
      ? patch.notes.slice(-10)
      : Array.isArray(fallback.notes)
        ? fallback.notes.slice(-10)
        : [],
  };
}

function upsertTrackedRequest(patch) {
  //Merge tracked request updates into state
  const normalized = normalizeTrackedPatch(patch);
  if (!normalized.id) return;

  const existing = findTrackedRequest(normalized.id);
  if (!existing) {
    state.trackedRequests.push(normalized);
    return;
  }

  Object.assign(existing, normalizeTrackedPatch(patch, existing));
}

function appendNote(request, message) {
  //Append timestamped note and keep recent history
  if (!request || !message) return;
  request.notes = [
    ...(request.notes || []),
    `${new Date().toLocaleString()}: ${message}`,
  ].slice(-10);
}

function appendTrackedNote(id, message) {
  const request = ensureTrackedRequest(id, state.auth?.email || "");
  appendNote(request, message);
}

function updateTrackedStatus(id, status, note) {
  const request = ensureTrackedRequest(id);
  if (!request) return;

  request.status = status;
  request.updatedAt = new Date().toISOString();
  appendNote(request, note);
}

function latestNote(item) {
  return Array.isArray(item?.notes) && item.notes.length
    ? item.notes[item.notes.length - 1]
    : "";
}

function syncTrackedFromApiItems(items) {
  //Sync tracked fields from backend queue payload
  items.forEach((item) => {
    const id = requestIdFromApi(item);
    if (!id) return;

    const existing = findTrackedRequest(id);
    upsertTrackedRequest({
      id,
      ownerEmail: existing?.ownerEmail || "",
      activityDescription:
        item.ActivityDescription || existing?.activityDescription || "",
      environmentalPermitId: Number(
        item.EnvironmentalPermitID || existing?.environmentalPermitId || 0,
      ),
      permitFee: Number(item.PermitFee || existing?.permitFee || 0),
      status: latestStatusFromApi(item) || existing?.status || "",
      permitCreated: Boolean(item.Permit) || existing?.permitCreated,
      updatedAt: new Date().toISOString(),
    });
  });
}

function requestIdFromApi(item) {
  return Number(item?.ID || item?.id || 0);
}

function latestStatusFromApi(item) {
  //Extract newest status from status history list
  const statuses = Array.isArray(item?.Statuses) ? item.Statuses : [];
  if (!statuses.length) return "";

  const latest = statuses.reduce((max, current) =>
    Number(current?.ID || 0) > Number(max?.ID || 0) ? current : max,
  );
  return latest?.Status || "";
}

async function apiRequest(path, options = {}) {
  const method = options.method || "GET";
  const body = options.body;
  const skipAuth = Boolean(options.skipAuth);

  const headers = { Accept: "application/json" };
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }

  if (!skipAuth) {
    if (!state.auth?.token) {
      throw new Error("No active session.");
    }
    headers.Authorization = `Bearer ${state.auth.token}`;
  }

  //Fetch wrapper with auth header and consistent error parsing
  const response = await fetch(`/api${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    if (response.status === 401 && !skipAuth) {
      clearAuth();
      renderSessionState();
    }
    throw new Error(extractApiError(payload, response));
  }

  return payload;
}

function extractApiError(payload, response) {
  //Build readable error from backend payload
  const parts = [payload?.error, payload?.message, payload?.details].filter(
    Boolean,
  );
  if (parts.length) return parts.join(" | ");
  return `${response.status} ${response.statusText || "Request failed"}`;
}

function clearAuth() {
  state.auth = null;
  saveState();
}

function countByStatus(rows, status) {
  return rows.filter((item) => item.status === status).length;
}

function toDateOnlyISO(value) {
  if (!value) return "";
  const date = new Date(`${value}T00:00:00Z`);
  return Number.isNaN(date.getTime()) ? "" : date.toISOString();
}

function showNotice(message, level = "info") {
  //Show timed alert banner for success info and errors
  const banner = $("#app-notice");
  if (!banner) return;
  if (noticeTimer) clearTimeout(noticeTimer);

  const styles = {
    success: "border-env-500 bg-env-100 text-env-700",
    error: "border-warn-600 bg-warn-100 text-warn-600",
    info: "border-gov-300 bg-gov-100 text-gov-700",
  };

  banner.className = `mt-4 rounded-lg border px-4 py-3 text-sm ${styles[level] || styles.info}`;
  banner.textContent = message;
  noticeTimer = setTimeout(() => banner.classList.add("hidden"), 6000);
}

function escapeHtml(value) {
  //Escape html before injecting into markup
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}
