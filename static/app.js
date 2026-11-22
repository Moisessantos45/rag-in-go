const state = {
    think: false,
    topK: 5,
    minSimilarity: 0.0,
    documentFilter: "",
    includeContexts: true,
    isStreaming: false
};

let userScrolledUp = false;

document.addEventListener("DOMContentLoaded", () => {
    loadSettingsFromStorage();
    updateUIFromSettings();
    setupDrawerDropzone();
    setupScrollListener();
    loadDocuments();
    lucide.createIcons();
});

function loadSettingsFromStorage() {
    try {
        const saved = localStorage.getItem("rag_settings");
        if (saved) {
            const parsed = JSON.parse(saved);
            state.think = parsed.think ?? false;
            state.topK = parsed.topK ?? 5;
            state.minSimilarity = parsed.minSimilarity ?? 0.0;
            state.includeContexts = parsed.includeContexts ?? true;
        }
    } catch (e) {}
    state.documentFilter = "";
}

function saveSettingsToStorage() {
    const toSave = {
        think: state.think,
        topK: state.topK,
        minSimilarity: state.minSimilarity,
        includeContexts: state.includeContexts
    };
    localStorage.setItem("rag_settings", JSON.stringify(toSave));
}

function updateUIFromSettings() {
    const thinkSetting = document.getElementById("setting-think");
    if (thinkSetting) thinkSetting.checked = state.think;

    const topkSetting = document.getElementById("setting-topk");
    if (topkSetting) {
        topkSetting.value = state.topK;
        document.getElementById("topk-val").innerText = state.topK;
    }

    const minsimSetting = document.getElementById("setting-minsim");
    if (minsimSetting) {
        minsimSetting.value = state.minSimilarity;
        document.getElementById("minsim-val").innerText = parseFloat(state.minSimilarity).toFixed(2);
    }

    const docSelect = document.getElementById("setting-doc-filter");
    if (docSelect) docSelect.value = state.documentFilter;

    const heroPill = document.getElementById("hero-doc-pill");
    if (heroPill) heroPill.innerText = state.documentFilter || "Todos los documentos";

    const chatPill = document.getElementById("chat-doc-pill");
    if (chatPill) chatPill.innerText = state.documentFilter || "Todos los documentos";

    const heroThinkPill = document.getElementById("hero-think-pill");
    if (heroThinkPill) {
        if (state.think) {
            heroThinkPill.classList.remove("hidden");
            heroThinkPill.classList.add("flex");
        } else {
            heroThinkPill.classList.add("hidden");
            heroThinkPill.classList.remove("flex");
        }
    }
}

function openSettingsModal() {
    const modal = document.getElementById("settings-modal");
    const card = document.getElementById("settings-card");
    modal.classList.remove("hidden");
    setTimeout(() => {
        modal.classList.remove("opacity-0");
        card.classList.remove("scale-95");
    }, 10);
}

function closeSettingsModal() {
    const modal = document.getElementById("settings-modal");
    const card = document.getElementById("settings-card");
    modal.classList.add("opacity-0");
    card.classList.add("scale-95");
    setTimeout(() => {
        modal.classList.add("hidden");
    }, 200);
}

function saveSettings() {
    state.think = document.getElementById("setting-think").checked;
    state.topK = parseInt(document.getElementById("setting-topk").value) || 5;
    state.minSimilarity = parseFloat(document.getElementById("setting-minsim").value) || 0.0;
    state.documentFilter = document.getElementById("setting-doc-filter").value;

    saveSettingsToStorage();
    updateUIFromSettings();
    closeSettingsModal();
}

function openDocDrawer() {
    const drawer = document.getElementById("doc-drawer");
    const card = document.getElementById("doc-drawer-card");
    drawer.classList.remove("hidden");
    setTimeout(() => {
        drawer.classList.remove("opacity-0");
        card.classList.remove("translate-x-full");
    }, 10);
    loadDocuments();
}

function closeDocDrawer() {
    const drawer = document.getElementById("doc-drawer");
    const card = document.getElementById("doc-drawer-card");
    drawer.classList.add("opacity-0");
    card.classList.add("translate-x-full");
    setTimeout(() => {
        drawer.classList.add("hidden");
    }, 250);
}

function filterByDocument(docName) {
    state.documentFilter = docName;
    saveSettingsToStorage();
    updateUIFromSettings();
    closeDocDrawer();
}

let isUploading = false;

function triggerFileInput() {
    if (isUploading) return;
    document.getElementById("hidden-file-input").click();
}

async function handleFileSelected(input) {
    if (!input.files || input.files.length === 0 || isUploading) return;
    const file = input.files[0];
    isUploading = true;

    const badge = document.getElementById("hero-upload-badge");
    const text = document.getElementById("hero-upload-text");
    const drawerStatus = document.getElementById("drawer-upload-status");
    const drawerBtn = document.getElementById("drawer-upload-btn");

    if (badge && text) {
        text.innerHTML = `<span class="inline-block animate-spin mr-1.5">⏳</span> Indexando <strong>${file.name}</strong>... por favor espera`;
        badge.classList.remove("hidden");
    }

    if (drawerStatus) {
        drawerStatus.innerHTML = `<div class="p-2.5 rounded-xl bg-slate-100 text-slate-700 text-xs flex items-center gap-2 border border-slate-200"><span class="animate-spin">⏳</span> Indexando <strong>${file.name}</strong>...</div>`;
        drawerStatus.classList.remove("hidden");
    }
    if (drawerBtn) drawerBtn.disabled = true;

    const customName = document.getElementById("drawer-doc-name")?.value?.trim() || "";
    const formData = new FormData();
    formData.append("document", file);
    if (customName) {
        formData.append("name", customName);
    }

    try {
        const resp = await fetch("/upload", {
            method: "POST",
            body: formData
        });
        const data = await resp.json();
        if (resp.ok) {
            const successMsg = `¡Completado! ${data.name} (${data.chunks_total} chunks)`;
            if (text) text.innerHTML = `<span class="text-emerald-600 font-bold">✓</span> ${successMsg}`;
            if (drawerStatus) drawerStatus.innerHTML = `<div class="p-2.5 rounded-xl bg-emerald-50 text-emerald-800 text-xs border border-emerald-200">✓ ${successMsg}</div>`;
            input.value = "";
            if (document.getElementById("drawer-doc-name")) document.getElementById("drawer-doc-name").value = "";
            setTimeout(() => {
                if (badge) badge.classList.add("hidden");
                if (drawerStatus) drawerStatus.classList.add("hidden");
            }, 3500);
            loadDocuments();
        } else {
            const errMsg = data.error || "Error al indexar archivo";
            if (text) text.innerHTML = `<span class="text-rose-600 font-bold">✗</span> ${errMsg}`;
            if (drawerStatus) drawerStatus.innerHTML = `<div class="p-2.5 rounded-xl bg-rose-50 text-rose-800 text-xs border border-rose-200">✗ ${errMsg}</div>`;
        }
    } catch (e) {
        if (text) text.innerText = "Error de red subiendo archivo";
        if (drawerStatus) drawerStatus.innerHTML = `<div class="p-2.5 rounded-xl bg-rose-50 text-rose-800 text-xs border border-rose-200">✗ Error de red</div>`;
    } finally {
        isUploading = false;
        if (drawerBtn) drawerBtn.disabled = false;
    }
}

function setupDrawerDropzone() {
    const dropzone = document.getElementById("drawer-dropzone");
    if (!dropzone) return;

    dropzone.addEventListener("click", () => triggerFileInput());
}

async function submitDrawerUpload() {
    triggerFileInput();
}

async function loadDocuments() {
    try {
        const resp = await fetch("/documents");
        const data = await resp.json();
        const navCount = document.getElementById("nav-doc-count");
        const drawerList = document.getElementById("drawer-docs-list");
        const select = document.getElementById("setting-doc-filter");

        const count = (data.documents && data.documents.length) || 0;
        if (navCount) navCount.innerText = count;

        if (data.documents && data.documents.length > 0) {
            if (drawerList) {
                drawerList.innerHTML = data.documents.map(d => `
                    <div class="p-3 rounded-2xl border border-slate-200/80 bg-slate-50 flex items-center justify-between gap-2">
                        <div class="min-w-0">
                            <p class="text-xs font-bold text-slate-900 truncate">${d.filename}</p>
                            <span class="text-[11px] text-slate-500 font-mono">${d.chunk_count} chunks</span>
                        </div>
                        <button onclick="filterByDocument('${d.filename}')" class="text-xs px-3 py-1 rounded-xl bg-white border border-slate-200 hover:bg-slate-100 text-slate-800 font-semibold shadow-2xs transition">
                            Usar
                        </button>
                    </div>
                `).join("");
            }

            if (select) {
                select.innerHTML = `<option value="">Todos los documentos</option>` + data.documents.map(d => `
                    <option value="${d.filename}" ${state.documentFilter === d.filename ? 'selected' : ''}>${d.filename}</option>
                `).join("");
            }
        }
    } catch (e) {}
}

function setupScrollListener() {
    const container = document.getElementById("chat-messages");
    if (!container) return;
    container.addEventListener("scroll", () => {
        const threshold = 120;
        const distanceFromBottom = container.scrollHeight - container.scrollTop - container.clientHeight;
        userScrolledUp = distanceFromBottom > threshold;
    });
}

function scrollToBottom(force = false) {
    const container = document.getElementById("chat-messages");
    if (!container) return;
    if (force || !userScrolledUp) {
        container.scrollTop = container.scrollHeight;
    }
}

function handleHeroKeyDown(e) {
    if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        submitFromHero();
    }
}

function handleChatKeyDown(e) {
    if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        submitFromChat();
    }
}

function askPreset(question) {
    switchToChatView();
    processQuery(question);
}

function submitFromHero() {
    const input = document.getElementById("hero-prompt-input");
    const query = input.value.trim();
    if (!query) return;

    input.value = "";
    switchToChatView();
    processQuery(query);
}

function submitFromChat() {
    const input = document.getElementById("chat-prompt-input");
    const query = input.value.trim();
    if (!query) return;

    input.value = "";
    input.style.height = "auto";
    processQuery(query);
}

function switchToChatView() {
    const welcome = document.getElementById("welcome-view");
    const chat = document.getElementById("chat-view");
    if (welcome) welcome.classList.add("hidden");
    if (chat) chat.classList.remove("hidden");
    lucide.createIcons();
}

function clearChat() {
    state.documentFilter = "";
    updateUIFromSettings();

    const welcome = document.getElementById("welcome-view");
    const chat = document.getElementById("chat-view");
    const messages = document.getElementById("chat-messages");

    if (messages) messages.innerHTML = "";
    if (chat) chat.classList.add("hidden");
    if (welcome) welcome.classList.remove("hidden");

    const heroInput = document.getElementById("hero-prompt-input");
    if (heroInput) {
        heroInput.value = "";
        heroInput.focus();
    }
    lucide.createIcons();
}

function formatMarkdown(text) {
    if (!text) return "";

    let cleaned = text.replace(/(?:\r?\n)+\s*\[Fuente:\s*([^\]]+)\]/gi, " [Fuente: $1]");
    cleaned = cleaned.replace(/(?:\r?\n)+\s*\[Fragmento\s*([^\]]+)\]/gi, " [Fragmento $1]");

    let formatted = marked.parse(cleaned);

    formatted = formatted.replace(/\[Fuente:\s*([^\]]+)\]/gi, (match, p1) => {
        return `<span class="source-badge">📄 ${p1.trim()}</span>`;
    });
    formatted = formatted.replace(/\[Fragmento\s*([^\]]+)\]/gi, (match, p1) => {
        return `<span class="source-badge">🧩 #${p1.trim()}</span>`;
    });

    return DOMPurify.sanitize(formatted);
}

async function processQuery(query) {
    if (state.isStreaming) return;

    appendUserMessage(query);

    const aiMessageId = "msg-" + Date.now();
    appendAIMessageContainer(aiMessageId);

    state.isStreaming = true;
    const sendBtn = document.getElementById("chat-send-btn");
    if (sendBtn) sendBtn.disabled = true;

    try {
        const payload = {
            query: query,
            document: state.documentFilter,
            top_k: state.topK,
            min_similarity: state.minSimilarity,
            generate: true,
            stream: true,
            think: state.think,
            include_contexts: state.includeContexts
        };

        const response = await fetch("/query", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(payload)
        });

        if (!response.ok) {
            const err = await response.json();
            updateAIMessageContent(aiMessageId, err.error || "Error procesando consulta", false);
            return;
        }

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";
        let accumulatedText = "";

        while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split("\n\n");
            buffer = lines.pop();

            for (const line of lines) {
                if (line.startsWith("event: meta")) {
                    const dataStr = line.replace("event: meta\ndata: ", "");
                    try {
                        const meta = JSON.parse(dataStr);
                        renderAIMeta(aiMessageId, meta);
                    } catch (e) {}
                } else if (line.startsWith("event: token")) {
                    const dataStr = line.replace("event: token\ndata: ", "");
                    try {
                        const parsed = JSON.parse(dataStr);
                        accumulatedText += parsed.token;
                        updateAIMessageContent(aiMessageId, accumulatedText, true);
                    } catch (e) {}
                } else if (line.startsWith("event: done")) {
                    const dataStr = line.replace("event: done\ndata: ", "");
                    try {
                        const parsed = JSON.parse(dataStr);
                        if (parsed.answer) {
                            accumulatedText = parsed.answer;
                            updateAIMessageContent(aiMessageId, accumulatedText, false);
                        }
                    } catch (e) {}
                } else if (line.startsWith("event: error")) {
                    const dataStr = line.replace("event: error\ndata: ", "");
                    try {
                        const parsed = JSON.parse(dataStr);
                        accumulatedText = parsed.error || "Error en el modelo";
                        updateAIMessageContent(aiMessageId, accumulatedText, false);
                    } catch (e) {}
                }
            }
        }

        updateAIMessageContent(aiMessageId, accumulatedText, false);
    } catch (err) {
        updateAIMessageContent(aiMessageId, "Error de conexión con el servidor", false);
    } finally {
        state.isStreaming = false;
        if (sendBtn) sendBtn.disabled = false;
        lucide.createIcons();
    }
}

function appendUserMessage(text) {
    const container = document.getElementById("chat-messages");
    const div = document.createElement("div");
    div.className = "flex justify-end";
    div.innerHTML = `
        <div class="msg-user px-4 py-3 max-w-xl text-sm text-slate-900 font-medium">
            ${DOMPurify.sanitize(text)}
        </div>
    `;
    container.appendChild(div);
    scrollToBottom(true);
}

function appendAIMessageContainer(id) {
    const container = document.getElementById("chat-messages");
    const div = document.createElement("div");
    div.className = "flex justify-start";
    div.innerHTML = `
        <div class="msg-ai px-5 py-4 max-w-2xl w-full space-y-3">
            <div id="${id}-loading" class="flex items-center gap-2.5 text-xs text-slate-500 py-1">
                <div class="flex items-center gap-1">
                    <span class="w-1.5 h-1.5 rounded-full bg-slate-400 animate-bounce" style="animation-delay: 0ms"></span>
                    <span class="w-1.5 h-1.5 rounded-full bg-slate-400 animate-bounce" style="animation-delay: 150ms"></span>
                    <span class="w-1.5 h-1.5 rounded-full bg-slate-400 animate-bounce" style="animation-delay: 300ms"></span>
                </div>
                <span>Pensando...</span>
            </div>
            <div id="${id}-body" class="markdown-body hidden">
                <span class="streaming-cursor"></span>
            </div>
            <div id="${id}-footer" class="hidden pt-2 border-t border-slate-100 flex items-center justify-between text-xs text-slate-400"></div>
            <div id="${id}-contexts-drawer" class="hidden pt-2 border-t border-slate-100"></div>
        </div>
    `;
    container.appendChild(div);
    lucide.createIcons();
    userScrolledUp = false;
    scrollToBottom(true);
}

function renderAIMeta(id, meta) {
    const footerDiv = document.getElementById(`${id}-footer`);
    if (!footerDiv) return;

    footerDiv.classList.remove("hidden");

    const uniqueSources = meta.sources ? [...new Set(meta.sources)] : [];
    const sourcesText = uniqueSources.length > 0 ? `<span class="text-slate-600 font-mono text-[11px] font-semibold">${uniqueSources.join(", ")}</span>` : "";

    footerDiv.innerHTML = `
        <div class="flex items-center gap-1.5 text-[11px] text-slate-400">
            <span>Fuente:</span>
            ${sourcesText}
        </div>
        ${meta.contexts && meta.contexts.length > 0 ? `
            <button onclick="toggleContextsDrawer('${id}')" class="text-[11px] text-slate-500 hover:text-slate-900 transition flex items-center gap-1 px-2 py-0.5 rounded-md bg-slate-50 border border-slate-200">
                <i data-lucide="layers" class="w-3 h-3"></i> ${meta.contexts.length} fragmentos
            </button>
        ` : ""}
    `;

    if (meta.contexts && meta.contexts.length > 0) {
        const drawer = document.getElementById(`${id}-contexts-drawer`);
        drawer.innerHTML = `
            <div class="space-y-1.5 mt-2">
                <p class="text-[10px] font-bold text-slate-400 uppercase tracking-wider">Fragmentos de Contexto:</p>
                <div class="space-y-1.5 max-h-40 overflow-y-auto pr-1">
                    ${meta.contexts.map((c, i) => `
                        <div class="p-2 rounded-xl bg-slate-50 border border-slate-200 text-xs text-slate-700 font-mono">
                            <span class="text-slate-900 font-bold block text-[10px] mb-0.5">#${i + 1} (${meta.sources[i] || 'Doc'}):</span>
                            ${DOMPurify.sanitize(c)}
                        </div>
                    `).join("")}
                </div>
            </div>
        `;
    }

    lucide.createIcons();
}

function toggleContextsDrawer(id) {
    const drawer = document.getElementById(`${id}-contexts-drawer`);
    if (drawer) {
        drawer.classList.toggle("hidden");
    }
}

function updateAIMessageContent(id, text, isStreaming) {
    const loading = document.getElementById(`${id}-loading`);
    const body = document.getElementById(`${id}-body`);
    if (!body) return;

    if (text) {
        if (loading) loading.classList.add("hidden");
        body.classList.remove("hidden");
        body.innerHTML = formatMarkdown(text) + (isStreaming ? `<span class="streaming-cursor"></span>` : "");
    } else if (isStreaming) {
        if (loading) loading.classList.remove("hidden");
    }

    scrollToBottom(false);
}
