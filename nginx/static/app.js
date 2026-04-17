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

function normalizeFinalDecision(value) {
  //Keep decision values constrained to explicit EO final outcomes.
  const decision = String(value || "");
  return decision === STATUS.accepted || decision === STATUS.rejected
    ? decision
    : "";
}

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
  bindAccountSettingsControls();
  bindTabControls();
  bindAddressSearch(
    "#register-address-search",
    "#register-address-results",
    "#register-form textarea[name='organization_address']",
    "#register-address-search-btn",
  );
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
  const ownerEmail = normalizeEmail(
    item?.ownerEmail || item?.owner_email || item?.RegulatedEntity?.email,
  );
  const environmentalPermit =
    item?.EnvironmentalPermit || item?.environmentalPermit || {};
  return {
    id: Number(item?.id || 0),
    ownerEmail,
    ownerName:
      item?.ownerName ||
      item?.owner_name ||
      item?.RegulatedEntity?.contact_person_name ||
      "",
    organizationName:
      item?.organizationName ||
      item?.organization_name ||
      item?.RegulatedEntity?.organization_name ||
      "",
    permitName:
      item?.permitName ||
      item?.permit_name ||
      environmentalPermit?.PermitName ||
      environmentalPermit?.permit_name ||
      "",
    permitDescription:
      item?.permitDescription ||
      item?.permit_description ||
      environmentalPermit?.Description ||
      environmentalPermit?.description ||
      "",
    activityDescription: String(
      item?.activityDescription || item?.ActivityDescription || "",
    ),
    activitySite: String(item?.activitySite || item?.ActivitySite || ""),
    activityStartDate: String(
      item?.activityStartDate || item?.ActivityStartDate || "",
    ),
    activityDuration: Number(
      item?.activityDuration || item?.ActivityDuration || 0,
    ),
    environmentalPermitId: Number(item?.environmentalPermitId || 0),
    permitFee: Number(item?.permitFee || 0),
    status: String(item?.status || ""),
    finalDecision: normalizeFinalDecision(item?.finalDecision),
    finalDecisionDescription: String(
      item?.finalDecisionDescription ||
        item?.final_decision_description ||
        item?.Decision?.Description ||
        "",
    ),
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

//End the active session and refresh role-scoped UI state.
function endSession(message = "Session ended.") {
  closeAccountSettings();
  clearAuth();
  renderSessionState();
  showNotice(message, "info");
}

function bindSessionControls() {
  $("#logout-btn")?.addEventListener("click", () => endSession());

  const topAuthLink = $("#top-auth-link");
  if (topAuthLink && topAuthLink.dataset.bound !== "true") {
    topAuthLink.dataset.bound = "true";
    topAuthLink.addEventListener("click", (event) => {
      //Allow normal navigation to login page when no session exists.
      if (!state.auth?.token) return;

      event.preventDefault();
      endSession("Session ended. Sign in to switch accounts.");
    });
  }
}

function renderTopAuthLink() {
  const topAuthLink = $("#top-auth-link");
  if (!topAuthLink) return;

  const hasSession = Boolean(state.auth?.token);
  topAuthLink.textContent = hasSession ? "Logout" : "Login";
  topAuthLink.setAttribute("href", hasSession ? "#" : "/login.html");
}

function bindAccountSettingsControls() {
  const button = $("#account-settings-btn");
  const closeButton = $("#account-settings-close");
  const backdrop = $("#account-settings-backdrop");
  const modal = $("#account-settings-modal");
  if (!button || !modal) return;
  if (button.dataset.bound === "true") return;

  button.dataset.bound = "true";

  button.addEventListener("click", async () => {
    if (!state.auth?.token) {
      showNotice("Sign in to access account settings.", "info");
      return;
    }

    await openAccountSettings();
  });

  closeButton?.addEventListener("click", closeAccountSettings);
  backdrop?.addEventListener("click", closeAccountSettings);
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !modal.classList.contains("hidden")) {
      closeAccountSettings();
    }
  });
}

async function openAccountSettings() {
  const uiRole = ACCOUNT_TYPE_TO_UI_ROLE[state.auth?.accountType];
  const modal = $("#account-settings-modal");
  const content = $("#account-settings-content");
  if (!uiRole || !modal || !content) return;

  modal.classList.remove("hidden");
  modal.classList.add("flex");
  document.body.classList.add("overflow-hidden");
  content.innerHTML =
    '<p class="text-sm text-slate-600">Loading account settings...</p>';

  //Map each role to its settings fragment and initializer.
  const settingsByRole = {
    re: {
      fragmentPath: "/fragments/re-account-settings.html",
      initPanel: initReAccountSettingsPanel,
    },
    eo: {
      fragmentPath: "/fragments/eo-account-settings.html",
      initPanel: initEoAccountSettingsPanel,
    },
  };
  const settings = settingsByRole[uiRole];
  if (!settings) return;

  try {
    const response = await fetch(settings.fragmentPath, {
      method: "GET",
      headers: { Accept: "text/html" },
    });

    if (!response.ok) {
      throw new Error(
        `${response.status} ${response.statusText || "Failed to load settings"}`,
      );
    }

    content.innerHTML = await response.text();
    await settings.initPanel();
  } catch (error) {
    content.innerHTML = `<p class="text-sm text-warn-600">${escapeHtml(
      error?.message || "Unable to load account settings.",
    )}</p>`;
  }
}

function closeAccountSettings() {
  const modal = $("#account-settings-modal");
  const content = $("#account-settings-content");
  if (!modal) return;

  modal.classList.add("hidden");
  modal.classList.remove("flex");
  document.body.classList.remove("overflow-hidden");
  if (content) content.innerHTML = "";
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

  if (state.auth?.token) {
    showNotice(
      "You are already signed in. Logout from the dashboard top bar before switching accounts.",
      "info",
    );
    if (!window.location.pathname.endsWith("/dashboard.html")) {
      window.location.assign("/dashboard.html");
    }
    return;
  }

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

    if (!window.location.pathname.endsWith("/dashboard.html")) {
      window.location.assign("/dashboard.html");
      return;
    }

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

function showDashboardLoggedOutState(
  panel,
  content,
  loginRequired,
  accountSettingsButton,
) {
  //Shared dashboard fallback when there is no valid signed-in session.
  closeAccountSettings();
  accountSettingsButton?.classList.add("hidden");
  loginRequired?.classList.remove("hidden");
  hideAppPanel(panel, content);
}

function renderSessionState() {
  const panel = $("#app-panel");
  const sessionTitle = $("#session-title");
  const content = $("#tab-content");
  const loginRequired = $("#dashboard-login-required");
  const accountSettingsButton = $("#account-settings-btn");

  renderTopAuthLink();

  if (!panel || !sessionTitle || !content) return;

  if (!state.auth?.token) {
    showDashboardLoggedOutState(
      panel,
      content,
      loginRequired,
      accountSettingsButton,
    );
    return;
  }

  const uiRole = ACCOUNT_TYPE_TO_UI_ROLE[state.auth.accountType];
  if (!uiRole) {
    clearAuth();
    showDashboardLoggedOutState(
      panel,
      content,
      loginRequired,
      accountSettingsButton,
    );
    return;
  }

  loginRequired?.classList.add("hidden");
  accountSettingsButton?.classList.remove("hidden");

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

function renderDefinitionRows(container, rows) {
  //Render a compact definition list used by account overview panels.
  container.innerHTML = rows
    .map(
      ([label, value]) =>
        `<div><dt class="text-slate-500">${label}</dt><dd>${escapeHtml(value)}</dd></div>`,
    )
    .join("");
}

function bindPasswordChangeForm(selector) {
  //Shared password change flow for account settings forms.
  bindSubmit(selector, async (data, form) => {
    if (String(data.new_password || "").length < 6) {
      showNotice("New password must be at least six characters.", "error");
      return;
    }
    if (data.new_password !== data.confirm_new_password) {
      showNotice("New password and confirmation must match.", "error");
      return;
    }

    await apiRequest("/change-password", {
      method: "POST",
      body: {
        current_password: data.current_password,
        new_password: data.new_password,
      },
    });

    form.reset();
    showNotice("Password updated successfully.", "success");
  });
}

async function initReAccountTab() {
  const profile = $("#re-profile");
  const note = $("#re-account-note");
  if (!profile) return;

  const renderAccount = (account) => {
    const rows = [
      ["Email", account?.email || state.auth?.email || ""],
      ["Account Type", "regulated_entity"],
      ["Contact", account?.contact_person_name || "n/a"],
      ["Organization", account?.organization_name || "n/a"],
      ["Address", account?.organization_address || "n/a"],
    ];

    renderDefinitionRows(profile, rows);
  };

  let account;
  try {
    account = await apiRequest("/account");
  } catch (error) {
    profile.innerHTML =
      '<p class="text-sm text-slate-600">Unable to load account details.</p>';
    if (note) note.textContent = "Try refreshing this tab";
    showNotice(`Failed to load account details: ${error.message}`, "error");
    return;
  }

  if (account?.account_type !== ACCOUNT_TYPES.re) {
    profile.innerHTML =
      '<p class="text-sm text-slate-600">Regulated entity account details are unavailable for this session.</p>';
    if (note) note.textContent = "Sign in as a regulated entity";
    return;
  }

  renderAccount(account);
  if (note) {
    note.textContent =
      "Use Account in the top navigation bar to edit this information.";
  }
}

async function initReAccountSettingsPanel() {
  const root = $("#account-settings-content");
  if (!root) return;
  const profile = $("#re-settings-profile", root);
  const note = $("#re-settings-note", root);
  const accountForm = $("#re-settings-account-form", root);
  if (!profile || !accountForm) return;

  const renderAccount = (account) => {
    const rows = [
      ["Email", account?.email || state.auth?.email || ""],
      ["Account Type", "regulated_entity"],
      ["Contact", account?.contact_person_name || "n/a"],
      ["Organization", account?.organization_name || "n/a"],
      ["Address", account?.organization_address || "n/a"],
    ];

    renderDefinitionRows(profile, rows);

    accountForm.elements.namedItem("contact_person_name").value =
      account?.contact_person_name || "";
    accountForm.elements.namedItem("email").value = account?.email || "";
    accountForm.elements.namedItem("organization_name").value =
      account?.organization_name || "";
    accountForm.elements.namedItem("organization_address").value =
      account?.organization_address || "";
  };

  let account;
  try {
    account = await apiRequest("/account");
  } catch (error) {
    profile.innerHTML =
      '<p class="text-sm text-slate-600">Unable to load account details.</p>';
    if (note) note.textContent = "Try reopening Account settings";
    showNotice(`Failed to load account details: ${error.message}`, "error");
    return;
  }

  if (account?.account_type !== ACCOUNT_TYPES.re) {
    profile.innerHTML =
      '<p class="text-sm text-slate-600">Regulated entity account details are unavailable for this session.</p>';
    if (note) note.textContent = "Sign in as a regulated entity";
    return;
  }

  renderAccount(account);
  if (note) {
    note.textContent =
      "Updates here sync to the read-only dashboard account overview.";
  }

  bindAddressSearch(
    "#re-settings-address-search",
    "#re-settings-address-results",
    "#re-settings-account-form textarea[name='organization_address']",
    "#re-settings-address-search-btn",
  );

  bindSubmit("#re-settings-account-form", async (data) => {
    const previousEmail = normalizeEmail(state.auth?.email);
    const updated = await apiRequest("/account", {
      method: "PATCH",
      body: {
        contact_person_name: data.contact_person_name,
        email: normalizeEmail(data.email),
        organization_name: data.organization_name,
        organization_address: data.organization_address,
      },
    });

    if (updated?.account_type !== ACCOUNT_TYPES.re) {
      throw new Error("Unexpected account type in update response.");
    }

    const nextEmail = normalizeEmail(updated.email || previousEmail);
    if (previousEmail && nextEmail && previousEmail !== nextEmail) {
      state.trackedRequests = state.trackedRequests.map((item) =>
        item.ownerEmail === previousEmail
          ? { ...item, ownerEmail: nextEmail }
          : item,
      );
      if (state.reProfiles[previousEmail] && !state.reProfiles[nextEmail]) {
        state.reProfiles[nextEmail] = state.reProfiles[previousEmail];
      }
      delete state.reProfiles[previousEmail];
    }

    upsertReProfile({
      email: nextEmail,
      contact_person_name: updated.contact_person_name,
      organization_name: updated.organization_name,
      organization_address: updated.organization_address,
    });

    state.auth = {
      ...state.auth,
      email: nextEmail,
    };
    saveState();
    renderSessionState();
    renderAccount(updated);
    showNotice("Regulated entity account updated.", "success");
  });

  bindPasswordChangeForm("#re-settings-password-form");
}

async function initRePermitTab() {
  await refreshMyPermitRequests();
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
    const activityDurationNs = Math.round(
      durationHours * 60 * 60 * 1000000000,
    );

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
        activity_site: data.activity_site,
        activity_start_date: startDateIso,
        //Convert hours to nanoseconds for backend duration format
        activity_duration: activityDurationNs,
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
      activitySite: data.activity_site,
      activityStartDate: startDateIso,
      activityDuration: activityDurationNs,
      environmentalPermitId: permitId,
      permitName:
        environmentalPermits.find((item) => item.id === permitId)
          ?.permit_name || "",
      permitDescription:
        environmentalPermits.find((item) => item.id === permitId)
          ?.description || "",
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

async function initRePaymentTab() {
  await refreshMyPermitRequests();
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

async function initReAckTab() {
  await refreshMyPermitRequests();
  const rows = trackedRequestsForCurrentRe();
  renderList(
    "#re-ack-list",
    rows,
    "No tracked workflow updates are available.",
    (item) => {
      const finalDecision = finalDecisionFromTrackedItem(item);
      const permitIssued = finalDecision === STATUS.accepted
        ? item?.permitCreated
          ? "Yes"
          : "Pending issuance"
        : finalDecision === STATUS.rejected
          ? "No"
          : "Not yet decided";

      return card(
        item.id,
        [
          detail("Owner", ownerDisplayName(item)),
          detail("EO final decision", finalDecision || "Not yet decided"),
          detail("Permit issued", permitIssued),
          detail("Current status", item.status || "Unknown"),
          detail("Latest note", latestNote(item) || "No note available"),
        ],
      );
    },
  );
}

async function initEoAccountTab() {
  const profile = $("#eo-profile");
  const note = $("#eo-account-note");
  if (!profile) return;

  const renderAccount = (account) => {
    const rows = [
      ["Email", account?.email || state.auth?.email || ""],
      ["Account Type", "environmental_officer"],
      ["Name", account?.name || "n/a"],
    ];

    renderDefinitionRows(profile, rows);
  };

  let account;
  try {
    account = await apiRequest("/account");
  } catch (error) {
    profile.innerHTML =
      '<p class="text-sm text-slate-600">Unable to load account details.</p>';
    if (note) note.textContent = "Try refreshing this tab";
    showNotice(`Failed to load account details: ${error.message}`, "error");
    return;
  }

  if (account?.account_type !== ACCOUNT_TYPES.eo) {
    profile.innerHTML =
      '<p class="text-sm text-slate-600">Environmental officer account details are unavailable for this session.</p>';
    if (note) note.textContent = "Sign in as an environmental officer";
    return;
  }

  renderAccount(account);
  if (note) {
    note.textContent =
      "Use Account in the top navigation bar to edit this information.";
  }
}

async function initEoAccountSettingsPanel() {
  const root = $("#account-settings-content");
  if (!root) return;
  const profile = $("#eo-settings-profile", root);
  const note = $("#eo-settings-note", root);
  const accountForm = $("#eo-settings-account-form", root);
  if (!profile || !accountForm) return;

  const renderAccount = (account) => {
    const rows = [
      ["Email", account?.email || state.auth?.email || ""],
      ["Account Type", "environmental_officer"],
      ["Name", account?.name || "n/a"],
    ];

    renderDefinitionRows(profile, rows);

    accountForm.elements.namedItem("name").value = account?.name || "";
    accountForm.elements.namedItem("email").value = account?.email || "";
  };

  let account;
  try {
    account = await apiRequest("/account");
  } catch (error) {
    profile.innerHTML =
      '<p class="text-sm text-slate-600">Unable to load account details.</p>';
    if (note) note.textContent = "Try reopening Account settings";
    showNotice(`Failed to load account details: ${error.message}`, "error");
    return;
  }

  if (account?.account_type !== ACCOUNT_TYPES.eo) {
    profile.innerHTML =
      '<p class="text-sm text-slate-600">Environmental officer account details are unavailable for this session.</p>';
    if (note) note.textContent = "Sign in as an environmental officer";
    return;
  }

  renderAccount(account);
  if (note) {
    note.textContent =
      "Updates here sync to the read-only dashboard account overview.";
  }

  bindSubmit("#eo-settings-account-form", async (data) => {
    const updated = await apiRequest("/account", {
      method: "PATCH",
      body: {
        name: data.name,
        email: normalizeEmail(data.email),
      },
    });

    if (updated?.account_type !== ACCOUNT_TYPES.eo) {
      throw new Error("Unexpected account type in update response.");
    }

    state.auth = {
      ...state.auth,
      email: normalizeEmail(updated.email || state.auth?.email),
    };
    saveState();
    renderSessionState();
    renderAccount(updated);
    showNotice("Environmental officer account updated.", "success");
  });

  bindPasswordChangeForm("#eo-settings-password-form");
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

async function initEoIssueTab() {
  //Show all permit applications before final decision actions.
  const refreshDecisionInputs = async () => {
    try {
      const payload = await apiRequest("/eo/permit-requests");
      const items = Array.isArray(payload.items) ? payload.items : [];
      syncTrackedFromApiItems(items);
      saveState();

      renderEoAllPermitRequests(items);
      renderKnownBeingReviewed();

      const beingReviewed = items.filter(
        (item) => latestStatusFromApi(item) === STATUS.beingReviewed,
      );
      setSelectOptions(
        "#eo-final-decision-form select[name='request_id']",
        "Select being-reviewed request",
        beingReviewed,
        (item) => requestIdFromApi(item),
        (item) =>
          `#${requestIdFromApi(item)} - ${item.ActivityDescription || "request"}`,
      );
    } catch (error) {
      renderList(
        "#eo-all-requests-list",
        [],
        "Unable to load permit applications.",
        () => "",
      );
      setSelectOptions(
        "#eo-final-decision-form select[name='request_id']",
        "Select being-reviewed request",
        [],
      );
      renderKnownBeingReviewed();
      showNotice(`Failed to load permit application details: ${error.message}`, "error");
    }
  };

  await refreshDecisionInputs();

  bindSubmit("#eo-final-decision-form", async (data, form) => {
    const requestId = Number(data.request_id);
    if (!requestId) {
      showNotice("Select a permit request from the Being Reviewed list.", "error");
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
    const tracked = ensureTrackedRequest(requestId);
    if (tracked) {
      tracked.permitCreated = decision === STATUS.accepted;
      tracked.finalDecision = normalizeFinalDecision(decision);
      tracked.finalDecisionDescription = data.description || "";
    }
    updateTrackedStatus(
      requestId,
      decision,
      `EO final decision: ${decision}. ${data.description || "No additional note."}`,
    );
    saveState();

    form.reset();
    await refreshDecisionInputs();
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

  //Always bind export actions so PDF/CSV buttons work even when no rows are present.
  bindEoReportExports();

  if (!rows.length) {
    table.innerHTML =
      '<tr><td class="px-2 py-2 text-slate-600" colspan="11">No tracked requests are available.</td></tr>';
    return;
  }

  table.innerHTML = rows
    .map(
      (item) => `
        <tr class="text-slate-800">
          <td class="px-2 py-2 font-mono text-xs">${escapeHtml(item.id)}</td>
          <td class="px-2 py-2">${escapeHtml(ownerDisplayName(item))}</td>
          <td class="px-2 py-2">${escapeHtml(item.ownerEmail || item?.RegulatedEntity?.email || "unknown")}</td>
          <td class="px-2 py-2">${escapeHtml(permitTypeDisplay(item))}</td>
          <td class="px-2 py-2">${escapeHtml(item.activityDescription || "n/a")}</td>
          <td class="px-2 py-2">${escapeHtml(item.environmentalPermitId || "n/a")}</td>
          <td class="px-2 py-2">${escapeHtml(item.status || "Unknown")}</td>
          <td class="px-2 py-2">${escapeHtml(item.finalDecision || "Not decided")}</td>
          <td class="px-2 py-2">${escapeHtml(item.finalDecisionDescription || "n/a")}</td>
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
          detail("Permit Type", permitTypeDisplay(item)),
          detail("Permit Fee", money(item.PermitFee || item.permitFee || 0)),
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
        detail("Activity Site", item.activitySite || "n/a"),
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
        detail("Owner", ownerDisplayName(item)),
        detail(
          "Owner Email",
          item.ownerEmail || item?.RegulatedEntity?.email || "unknown",
        ),
        detail("Activity Description", item.activityDescription || "n/a"),
        detail("Activity Start Date", formatActivityStartDate(item.activityStartDate)),
        detail("Activity Duration", formatActivityDuration(item.activityDuration)),
        detail("Permit Fee", money(item.permitFee || 0)),
        detail("Activity Site", item.activitySite || "n/a"),
      ]),
  );
}

function renderEoAllPermitRequests(items) {
  renderList(
    "#eo-all-requests-list",
    items,
    "No permit applications are available.",
    (item) => {
      const requestId = requestIdFromApi(item);
      return card(requestId || "n/a", [
        detail("Latest status", latestStatusFromApi(item) || "Unknown"),
        detail("Status history", statusHistoryFromApi(item) || "No status history"),
        detail("Regulated Entity ID", item.RegulatedEntityID || "n/a"),
        detail("Activity", item.ActivityDescription || "n/a"),
        detail("Activity Site", item.ActivitySite || "n/a"),
        detail("Activity Start Date", formatActivityStartDate(item.ActivityStartDate)),
        detail("Activity Duration", formatActivityDuration(item.ActivityDuration)),
        detail("Permit Template ID", item.EnvironmentalPermitID || "n/a"),
        detail("Permit Fee", money(item.PermitFee || 0)),
        detail("Final decision", item?.Decision?.Decision || "Not decided"),
        detail("Decision notes", item?.Decision?.Description || "n/a"),
      ]);
    },
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

function cacheReProfileFromApiItem(item) {
  const regulatedEntity =
    item?.RegulatedEntity ||
    item?.regulatedEntity ||
    item?.regulated_entity ||
    {};
  const email = normalizeEmail(
    regulatedEntity?.email || item?.ownerEmail || item?.owner_email,
  );
  if (!email) return;

  state.reProfiles[email] = {
    email,
    contact_person_name:
      regulatedEntity?.contact_person_name ||
      item?.ownerName ||
      item?.owner_name ||
      "",
    organization_name:
      regulatedEntity?.organization_name ||
      item?.organizationName ||
      item?.organization_name ||
      "",
    organization_address:
      regulatedEntity?.organization_address ||
      item?.organizationAddress ||
      item?.organization_address ||
      "",
  };
}

function ownerDisplayName(item) {
  const email = normalizeEmail(
    item?.ownerEmail || item?.owner_email || item?.RegulatedEntity?.email,
  );
  const profile = state.reProfiles[email] || {};
  const name =
    item?.ownerName ||
    item?.owner_name ||
    item?.RegulatedEntity?.contact_person_name ||
    profile.contact_person_name ||
    "";

  if (name && email) return `${name} <${email}>`;
  return name || email || "unknown";
}

function permitTypeDisplay(item) {
  const name =
    item?.permitName ||
    item?.permit_name ||
    item?.EnvironmentalPermit?.PermitName ||
    item?.EnvironmentalPermit?.permit_name ||
    "";
  const description =
    item?.permitDescription ||
    item?.permit_description ||
    item?.EnvironmentalPermit?.Description ||
    item?.EnvironmentalPermit?.description ||
    "";

  if (name && description) return `${name} — ${description}`;
  if (name) return name;
  if (description) return description;
  return `Permit #${item?.environmentalPermitId || "n/a"}`;
}

function workflowNotesFromApiItem(item) {
  const statuses = Array.isArray(item?.Statuses) ? item.Statuses : [];
  if (!statuses.length) return [];

  return statuses
    .slice()
    .sort((a, b) => Number(a?.ID || 0) - Number(b?.ID || 0))
    .map((status) => {
      const statusLabel = String(status?.Status || "").trim();
      const description = String(status?.Description || "").trim();
      if (!statusLabel && !description) return "";
      return description ? `${statusLabel}: ${description}` : statusLabel;
    })
    .filter(Boolean)
    .slice(-10);
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
    ownerName: "",
    organizationName: "",
    activityDescription: "",
    activitySite: "",
    activityStartDate: "",
    activityDuration: 0,
    environmentalPermitId: 0,
    permitName: "",
    permitDescription: "",
    permitFee: 0,
    status: "",
    finalDecision: "",
    finalDecisionDescription: "",
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
    ownerName: patch?.ownerName ?? fallback.ownerName ?? "",
    organizationName:
      patch?.organizationName ?? fallback.organizationName ?? "",
    permitName: patch?.permitName ?? fallback.permitName ?? "",
    permitDescription:
      patch?.permitDescription ?? fallback.permitDescription ?? "",
    activityDescription:
      patch?.activityDescription ?? fallback.activityDescription ?? "",
    activitySite: patch?.activitySite ?? fallback.activitySite ?? "",
    activityStartDate:
      patch?.activityStartDate ?? fallback.activityStartDate ?? "",
    activityDuration: Number(
      patch?.activityDuration ?? fallback.activityDuration ?? 0,
    ),
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
    finalDecision: normalizeFinalDecision(
      patch?.finalDecision !== undefined
        ? patch.finalDecision
        : fallback.finalDecision,
    ),
    finalDecisionDescription:
      patch?.finalDecisionDescription ?? fallback.finalDecisionDescription ?? "",
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
  if (status === STATUS.accepted || status === STATUS.rejected) {
    request.finalDecision = status;
  }
  request.updatedAt = new Date().toISOString();
  appendNote(request, note);
}

function latestNote(item) {
  if (Array.isArray(item?.notes) && item.notes.length) {
    return item.notes[item.notes.length - 1];
  }

  const workflowNotes = workflowNotesFromApiItem(item);
  return workflowNotes.length ? workflowNotes[workflowNotes.length - 1] : "";
}

function finalDecisionFromTrackedItem(item) {
  return (
    normalizeFinalDecision(item?.finalDecision) ||
    normalizeFinalDecision(item?.status)
  );
}

function syncTrackedFromApiItems(items) {
  //Sync tracked fields from backend queue payload
  items.forEach((item) => {
    const id = requestIdFromApi(item);
    if (!id) return;

    const existing = findTrackedRequest(id);
    const latestStatus = latestStatusFromApi(item) || existing?.status || "";
    const notes = workflowNotesFromApiItem(item);
    cacheReProfileFromApiItem(item);
    const environmentalPermit =
      item?.EnvironmentalPermit || item?.environmentalPermit || {};
    upsertTrackedRequest({
      id,
      ownerEmail:
        normalizeEmail(
          item?.ownerEmail || item?.owner_email || item?.RegulatedEntity?.email,
        ) ||
        existing?.ownerEmail ||
        "",
      ownerName:
        item?.ownerName ||
        item?.owner_name ||
        item?.RegulatedEntity?.contact_person_name ||
        existing?.ownerName ||
        "",
      organizationName:
        item?.organizationName ||
        item?.organization_name ||
        item?.RegulatedEntity?.organization_name ||
        existing?.organizationName ||
        "",
      permitName:
        item?.permitName ||
        item?.permit_name ||
        environmentalPermit?.PermitName ||
        environmentalPermit?.permit_name ||
        existing?.permitName ||
        "",
      permitDescription:
        item?.permitDescription ||
        item?.permit_description ||
        environmentalPermit?.Description ||
        environmentalPermit?.description ||
        existing?.permitDescription ||
        "",
      activityDescription:
        item.ActivityDescription || existing?.activityDescription || "",
      activitySite: item.ActivitySite || existing?.activitySite || "",
      activityStartDate:
        item.ActivityStartDate || existing?.activityStartDate || "",
      activityDuration: Number(
        item.ActivityDuration || existing?.activityDuration || 0,
      ),
      environmentalPermitId: Number(
        item.EnvironmentalPermitID || existing?.environmentalPermitId || 0,
      ),
      permitFee: Number(item.PermitFee || existing?.permitFee || 0),
      status: latestStatus,
      finalDecision:
        normalizeFinalDecision(item?.Decision?.Decision) ||
        normalizeFinalDecision(latestStatus) ||
        existing?.finalDecision ||
        "",
      finalDecisionDescription:
        item?.Decision?.Description ||
        latestStatusDescriptionFromApi(item) ||
        existing?.finalDecisionDescription ||
        "",
      permitCreated: Boolean(item.Permit) || existing?.permitCreated,
      notes: notes.length ? notes : existing?.notes || [],
      updatedAt: new Date().toISOString(),
    });
  });
}

async function refreshMyPermitRequests() {
  const payload = await apiRequest("/permit-requests");
  const items = Array.isArray(payload.items) ? payload.items : [];
  syncTrackedFromApiItems(items);
  saveState();
  return items;
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

function latestStatusDescriptionFromApi(item) {
  const statuses = Array.isArray(item?.Statuses) ? item.Statuses : [];
  if (!statuses.length) return "";

  const latest = statuses.reduce((max, current) =>
    Number(current?.ID || 0) > Number(max?.ID || 0) ? current : max,
  );
  return latest?.Description || "";
}

function statusHistoryFromApi(item) {
  const statuses = Array.isArray(item?.Statuses)
    ? [...item.Statuses]
    : [];
  if (!statuses.length) return "";

  statuses.sort((a, b) => Number(a?.ID || 0) - Number(b?.ID || 0));
  return statuses.map((status) => status?.Status || "").filter(Boolean).join(" -> ");
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

async function downloadAuthorizedFile(path, fallbackFilename) {
  if (!state.auth?.token) {
    throw new Error("No active session.");
  }

  const response = await fetch(`/api${path}`, {
    method: "GET",
    headers: {
      Accept: "*/*",
      Authorization: `Bearer ${state.auth.token}`,
    },
  });

  if (!response.ok) {
    const payload = await response.json().catch(() => ({}));
    throw new Error(extractApiError(payload, response));
  }

  const blob = await response.blob();
  const contentDisposition = response.headers.get("Content-Disposition") || "";
  const match = contentDisposition.match(/filename=\"?([^\";]+)\"?/i);
  const filename = match?.[1] || fallbackFilename;

  const href = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = href;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(href);
}

function bindEoReportExports() {
  const exportCsv = $("#eo-export-csv");
  const exportPdf = $("#eo-export-pdf");

  if (exportCsv && exportCsv.dataset.bound !== "true") {
    exportCsv.dataset.bound = "true";
    exportCsv.addEventListener("click", async () => {
      try {
        await downloadAuthorizedFile(
          "/eo/permit-requests/export.csv",
          "permit-applications.csv",
        );
        showNotice("CSV export downloaded.", "success");
      } catch (error) {
        showNotice(`CSV export failed: ${error.message}`, "error");
      }
    });
  }

  if (exportPdf && exportPdf.dataset.bound !== "true") {
    exportPdf.dataset.bound = "true";
    exportPdf.addEventListener("click", () => {
      const printableRows = sortedTrackedRequests();
      const printWindow = window.open(
        "about:blank",
        "_blank",
        "width=1100,height=800",
      );
      if (!printWindow) {
        showNotice("Allow popups to export the report as PDF.", "error");
        return;
      }

      const tableRows = printableRows
        .map(
          (item) => `
            <tr>
              <td>${escapeHtml(item.id)}</td>
              <td>${escapeHtml(ownerDisplayName(item))}</td>
              <td>${escapeHtml(item.ownerEmail || "unknown")}</td>
              <td>${escapeHtml(permitTypeDisplay(item))}</td>
              <td>${escapeHtml(item.activityDescription || "n/a")}</td>
              <td>${escapeHtml(item.environmentalPermitId || "n/a")}</td>
              <td>${escapeHtml(item.status || "Unknown")}</td>
              <td>${escapeHtml(item.finalDecision || "Not decided")}</td>
              <td>${escapeHtml(item.finalDecisionDescription || "n/a")}</td>
              <td>${escapeHtml(item.permitCreated ? "Created" : "Not created")}</td>
              <td>${escapeHtml(item.updatedAt ? new Date(item.updatedAt).toLocaleString() : "n/a")}</td>
            </tr>
          `,
        )
        .join("");

      const tableBody =
        tableRows ||
        '<tr><td colspan="11">No tracked requests are available.</td></tr>';

      const generatedAt = escapeHtml(new Date().toLocaleString());

      printWindow.document.open();
      printWindow.document.write(`
        <!doctype html>
        <html lang="en">
          <head>
            <meta charset="UTF-8" />
            <title>Environmental Officer Workflow Report</title>
            <style>
              body { font-family: Arial, sans-serif; padding: 24px; color: #1f2937; }
              h1 { margin: 0 0 8px 0; }
              p { margin: 0 0 16px 0; color: #4b5563; }
              .print-actions { margin: 0 0 12px 0; }
              .print-btn { border: 1px solid #94a3b8; background: #f8fafc; border-radius: 6px; padding: 8px 12px; cursor: pointer; }
              table { border-collapse: collapse; width: 100%; }
              th, td { border: 1px solid #cbd5e1; padding: 8px; text-align: left; font-size: 12px; }
              th { background: #f8fafc; }
              @media print {
                .print-actions { display: none; }
              }
            </style>
          </head>
          <body>
            <h1>Environmental Officer Workflow Report</h1>
            <p>Generated ${generatedAt}</p>
            <div class="print-actions">
              <button class="print-btn" type="button" onclick="window.print()">Print / Save as PDF</button>
            </div>
            <table>
              <thead>
                <tr>
                  <th>Request #</th>
                  <th>Owner</th>
                  <th>Owner Email</th>
                  <th>Permit Type</th>
                  <th>Activity</th>
                  <th>Permit Template ID</th>
                  <th>Status</th>
                  <th>Final Decision</th>
                  <th>Decision Notes</th>
                  <th>Permit Record</th>
                  <th>Updated</th>
                </tr>
              </thead>
              <tbody>${tableBody}</tbody>
            </table>
          </body>
        </html>
      `);
      printWindow.document.close();

      //Use both load and timeout triggers to handle browser-specific print timing.
      let hasPrinted = false;
      const triggerPrint = () => {
        if (hasPrinted || printWindow.closed) return;
        hasPrinted = true;
        printWindow.focus();
        printWindow.print();
      };

      printWindow.addEventListener(
        "load",
        () => {
          setTimeout(triggerPrint, 200);
        },
        { once: true },
      );

      setTimeout(triggerPrint, 900);
    });
  }
}

function bindAddressSearch(searchSelector, resultsSelector, targetSelector, buttonSelector) {
  const searchInput = $(searchSelector);
  const results = $(resultsSelector);
  const target = $(targetSelector);
  const button = $(buttonSelector);
  if (!searchInput || !results || !target || !button) return;
  if (button.dataset.bound === "true") return;

  button.dataset.bound = "true";

  results.addEventListener("change", () => {
    if (results.value) {
      target.value = results.value;
    }
  });

  button.addEventListener("click", async () => {
    const query = String(searchInput.value || "").trim();
    if (query.length < 3) {
      showNotice("Enter at least three characters for address suggestions.", "info");
      return;
    }

    button.disabled = true;
    results.innerHTML = "";
    try {
      const url = new URL("https://nominatim.openstreetmap.org/search");
      url.searchParams.set("q", query);
      url.searchParams.set("format", "jsonv2");
      url.searchParams.set("addressdetails", "1");
      url.searchParams.set("limit", "6");

      const response = await fetch(url.toString(), {
        method: "GET",
        headers: {
          Accept: "application/json",
        },
      });

      if (!response.ok) {
        throw new Error("Address lookup service unavailable");
      }

      const payload = await response.json();
      const suggestions = Array.isArray(payload)
        ? payload.map((item) => item?.display_name).filter(Boolean)
        : [];

      if (!suggestions.length) {
        results.innerHTML = '<option value="">No address suggestions found</option>';
        return;
      }

      results.innerHTML = suggestions
        .map(
          (value) => `<option value="${escapeHtml(value)}">${escapeHtml(value)}</option>`,
        )
        .join("");
      target.value = suggestions[0];
    } catch (error) {
      showNotice(`Address helper failed: ${error.message}`, "error");
    } finally {
      button.disabled = false;
    }
  });
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

function formatActivityStartDate(value) {
  if (!value) return "n/a";

  //Keep date-only values stable across timezones and very old years.
  const raw = String(value).trim();
  const match = raw.match(/^(\d{4})-(\d{2})-(\d{2})/);
  if (match) {
    const [, year, month, day] = match;
    return `${month}/${day}/${year}`;
  }

  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) return raw;

  return date.toLocaleDateString(undefined, {
    timeZone: "UTC",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  });
}

function formatActivityDuration(value) {
  const nanoseconds = Number(value || 0);
  if (!Number.isFinite(nanoseconds) || nanoseconds <= 0) return "n/a";

  const hours = nanoseconds / 3600000000000;
  if (hours >= 1) {
    const rounded = Number.isInteger(hours) ? hours.toFixed(0) : hours.toFixed(2);
    return `${rounded} hour${Number(rounded) === 1 ? "" : "s"}`;
  }

  const minutes = nanoseconds / 60000000000;
  const roundedMinutes = Number.isInteger(minutes)
    ? minutes.toFixed(0)
    : minutes.toFixed(1);
  return `${roundedMinutes} minute${Number(roundedMinutes) === 1 ? "" : "s"}`;
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
