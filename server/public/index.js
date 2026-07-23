// Global State
let activeTrip = null;
let queue = [];
let activeExportTab = 'tms';

// DOM Elements
const dropzone = document.getElementById('dropzone');
const fileInput = document.getElementById('file-input');
const preprocessCheck = document.getElementById('preprocess-check');
const modelSelect = document.getElementById('model-select');

const landingView = document.getElementById('landing-view');
const processingView = document.getElementById('processing-view');
const resultsView = document.getElementById('results-view');

const logContent = document.getElementById('log-content');
const queueList = document.getElementById('queue-list');
const queueCount = document.getElementById('queue-count');

const themeToggle = document.getElementById('theme-toggle');

// ==========================================================================
// Initialization
// ==========================================================================

document.addEventListener('DOMContentLoaded', () => {
    // Load existing queue
    fetchQueue();

    // Event Listeners: Drag & Drop Ingestion
    dropzone.addEventListener('click', () => fileInput.click());
    dropzone.addEventListener('dragover', (e) => {
        e.preventDefault();
        dropzone.classList.add('dragover');
    });
    dropzone.addEventListener('dragleave', () => dropzone.classList.remove('dragover'));
    dropzone.addEventListener('drop', (e) => {
        e.preventDefault();
        dropzone.classList.remove('dragover');
        if (e.dataTransfer.files.length > 0) {
            handleFile(e.dataTransfer.files[0]);
        }
    });

    fileInput.addEventListener('change', (e) => {
        if (e.target.files.length > 0) {
            handleFile(e.target.files[0]);
        }
    });

    // Theme Toggle
    themeToggle.addEventListener('click', toggleTheme);

    // Tab Switching
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
            document.querySelectorAll('.tab-pane').forEach(p => p.classList.remove('active'));
            
            btn.classList.add('active');
            const targetTab = btn.getAttribute('data-tab');
            document.getElementById(targetTab).classList.add('active');
        });
    });

    // Export Selector
    document.querySelectorAll('.export-tab').forEach(btn => {
        btn.addEventListener('click', (e) => {
            document.querySelectorAll('.export-tab').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            activeExportTab = btn.getAttribute('data-export');
            renderExportCode();
        });
    });

    // Copy Code Button
    document.getElementById('copy-code-btn').addEventListener('click', () => {
        const codeElement = document.getElementById('export-code');
        navigator.clipboard.writeText(codeElement.innerText).then(() => {
            const btn = document.getElementById('copy-code-btn');
            const originalText = btn.innerHTML;
            btn.innerHTML = '<i class="fa-solid fa-check"></i> Copied!';
            setTimeout(() => btn.innerHTML = originalText, 2000);
        });
    });

    // Close Results View Button
    document.getElementById('close-results').addEventListener('click', () => {
        resultsView.classList.remove('active');
        landingView.classList.add('active');
        document.querySelectorAll('.queue-item').forEach(item => item.classList.remove('active'));
        activeTrip = null;
    });
});

// ==========================================================================
// Theme Logic
// ==========================================================================

function toggleTheme() {
    const html = document.documentElement;
    const isDark = html.classList.toggle('dark');
    themeToggle.innerHTML = isDark ? '<i class="fa-solid fa-sun"></i>' : '<i class="fa-solid fa-moon"></i>';
}

// ==========================================================================
// File Ingestion Pipeline
// ==========================================================================

function handleFile(file) {
    if (!['image/jpeg', 'image/png'].includes(file.type)) {
        alert('Unsupported file type. Please upload a JPEG or PNG image.');
        return;
    }

    // Switch view to Processing
    landingView.classList.remove('active');
    processingView.classList.add('active');
    logContent.innerHTML = '';

    // Simulate real-time backend pipeline logs
    appendLog('INFO', 'Initializing ingestion pipeline...');
    appendLog('INFO', `File received: ${file.name} (${formatBytes(file.size)})`);
    appendLog('INFO', `MIME type validated: ${file.type}`);

    const applyPreprocess = preprocessCheck.checked;
    if (applyPreprocess) {
        appendLog('WARNING', 'Preprocessing requested: applying grayscale and contrast enhancement...');
    }

    const selectedModel = modelSelect.value;

    setTimeout(() => {
        appendLog('INFO', 'Transmitting image bytes to Gemini VLM Extraction API...');
        appendLog('INFO', `Model target: ${selectedModel}`);
        
        // Actually perform upload
        uploadFile(file, applyPreprocess, selectedModel);
    }, 1200);
}

function appendLog(level, message) {
    const timestamp = new Date().toISOString().slice(11, 19);
    const logLine = document.createElement('div');
    logLine.className = 'log-line';
    
    let levelClass = 'info';
    if (level === 'WARNING') levelClass = 'warning';
    if (level === 'SUCCESS') levelClass = 'success';
    if (level === 'ERROR') levelClass = 'error';

    logLine.innerHTML = `
        <span class="log-time">[${timestamp}]</span>
        <span class="log-text ${levelClass}">${level}: ${message}</span>
    `;
    logContent.appendChild(logLine);
    logContent.scrollTop = logContent.scrollHeight;
}

function uploadFile(file, preprocess, modelName) {
    const formData = new FormData();
    formData.append('image', file);

    let url = `/api/v1/trips/extract?model=${encodeURIComponent(modelName)}`;
    if (preprocess) {
        url += '&preprocess=true';
    }

    const startTime = Date.now();

    fetch(url, {
        method: 'POST',
        body: formData
    })
    .then(async response => {
        const text = await response.text();
        if (!response.ok) {
            throw new Error(`Server returned error ${response.status}: ${text}`);
        }
        return JSON.parse(text);
    })
    .then(data => {
        const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);
        appendLog('SUCCESS', `VLM extraction complete in ${elapsed}s`);
        appendLog('INFO', 'Executing deterministic validation guardrails...');
        
        // Render validation errors if they exist
        if (data.status === 'exception') {
            appendLog('ERROR', `Guardrail alert: routed to Exception queue (${data.validation.errors.length} failures)`);
            data.validation.errors.forEach(err => appendLog('ERROR', `  - ${err}`));
        } else {
            appendLog('SUCCESS', 'All validation checks passed successfully. Status set to Validated.');
        }

        setTimeout(() => {
            // Update queue list
            fetchQueue();
            
            // Show results
            showResults(data);
        }, 1500);
    })
    .catch(err => {
        appendLog('ERROR', `Ingestion failed: ${err.message}`);
        appendLog('INFO', 'System standing down. Press close to return.');
        
        // Add a back button in the logs on error
        const errDiv = document.createElement('div');
        errDiv.style.marginTop = '1rem';
        errDiv.innerHTML = `<button class="btn btn-secondary btn-sm" onclick="location.reload()">Back to Upload</button>`;
        logContent.appendChild(errDiv);
    });
}

// ==========================================================================
// Queue List & Sidebar Logic
// ==========================================================================

function fetchQueue() {
    fetch('/api/v1/trips')
        .then(res => res.json())
        .then(data => {
            queue = data || [];
            queueCount.innerText = queue.length;
            renderQueue();
        })
        .catch(err => console.error('Failed to fetch queue:', err));
}

function renderQueue() {
    if (queue.length === 0) {
        queueList.innerHTML = `
            <div class="empty-queue-msg text-dim">
                <i class="fa-solid fa-folder-open"></i>
                No extractions loaded.
            </div>
        `;
        return;
    }

    queueList.innerHTML = '';
    queue.forEach(item => {
        const dateStr = new Date(item.CreatedAt).toLocaleString([], {
            month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit'
        });

        const statusClass = item.Status === 'validated' ? 'validated' : 'exception';
        const activeClass = (activeTrip && activeTrip.ID === item.ID) ? 'active' : '';

        const card = document.createElement('div');
        card.className = `queue-item ${activeClass}`;
        card.innerHTML = `
            <div class="queue-item-meta">
                <span class="time">${dateStr}</span>
                <span class="status-badge ${statusClass}">${item.Status}</span>
            </div>
            <div class="queue-item-title">${item.LineItems ? item.LineItems.length : 0} legs extracted</div>
            <div class="queue-item-details">
                <span>Odo Open: ${item.OdometerOpen || 'N/A'}</span>
                <span>Close: ${item.OdometerClose || 'N/A'}</span>
            </div>
        `;

        card.addEventListener('click', () => {
            document.querySelectorAll('.queue-item').forEach(c => c.classList.remove('active'));
            card.classList.add('active');
            fetchTripDetail(item.ID);
        });

        queueList.appendChild(card);
    });
}

function fetchTripDetail(id) {
    fetch(`/api/v1/trips/${id}`)
        .then(res => res.json())
        .then(data => {
            // Adapt DB format structure back into response format structure for rendering
            const adapted = {
                status: data.Status,
                processed_at: data.CreatedAt,
                trip_sheet: {
                    odometer_open: data.OdometerOpen,
                    odometer_close: data.OdometerClose,
                    total_miles: data.TotalMiles,
                    confidence_score: data.ConfidenceScore,
                    flagged_fields: data.FlaggedFields || [],
                    line_items: (data.LineItems || []).map(li => ({
                        date: li.Date,
                        location: li.Location,
                        miles: li.Miles
                    }))
                },
                validation: {
                    errors: data.ValidationErrors || [],
                    odometer_delta_check: data.ValidationErrors && hasErrSubstring(data.ValidationErrors, "odometer") ? "fail" : "pass",
                    line_item_sum_check: data.ValidationErrors && hasErrSubstring(data.ValidationErrors, "line-item") ? "fail" : "pass",
                    confidence_check: data.ValidationErrors && hasErrSubstring(data.ValidationErrors, "confidence") ? "fail" : "pass"
                }
            };
            
            // Note: Since DB uses server path for local files, we serve the static file.
            // But we need to serve it properly.
            // On a real deployment, ImagePath would map to an S3 URL or relative directory.
            // We strip the local directory part so the image can be loaded in browser.
            // E.g., "/media/muhammad/FS/2026/POC-Trucking-Trip-Sheet-Automation/server/audit_images/xyz.jpg"
            // becomes "/audit_images/xyz.jpg" relative to Go server.
            let webPath = "";
            if (data.ImagePath) {
                // Find "audit_images" and make relative path
                const idx = data.ImagePath.indexOf("audit_images");
                if (idx !== -1) {
                    webPath = "/" + data.ImagePath.slice(idx);
                }
            }

            adapted.image_web_path = webPath;
            showResults(adapted);
        })
        .catch(err => console.error('Failed to fetch trip detail:', err));
}

function hasErrSubstring(errors, sub) {
    return errors.some(err => err.toLowerCase().includes(sub));
}

// ==========================================================================
// Results Rendering Logic
// ==========================================================================

function showResults(data) {
    activeTrip = data;
    
    // Switch views
    landingView.classList.remove('active');
    processingView.classList.remove('active');
    resultsView.classList.add('active');

    // 1. Render Audit Image
    // Use the uploaded file preview OR the server path
    const auditImg = document.getElementById('audit-image');
    if (data.image_web_path) {
        auditImg.src = data.image_web_path;
    } else {
        // Fallback: If we just uploaded, fetch the most recent image URL if available
        auditImg.src = "/test_data/images/sample3_clean.jpg"; // Default fallback
    }

    // 2. Render Status Metrics
    const statusPill = document.getElementById('extracted-status');
    statusPill.innerText = data.status;
    statusPill.className = `status-pill ${data.status === 'validated' ? 'green' : 'red'}`;

    const confScore = Math.round(data.trip_sheet.confidence_score * 100);
    document.getElementById('extracted-confidence').innerText = `${confScore}%`;

    // 3. Render Odometer Card
    document.getElementById('val-odo-open').innerText = formatNum(data.trip_sheet.odometer_open);
    document.getElementById('val-odo-close').innerText = formatNum(data.trip_sheet.odometer_close);
    document.getElementById('val-total-miles').innerText = formatNum(data.trip_sheet.total_miles);

    // Highlight flagged odometer fields
    toggleFlagged('val-odo-open', data.trip_sheet.flagged_fields.includes('odometer_open'));
    toggleFlagged('val-odo-close', data.trip_sheet.flagged_fields.includes('odometer_close'));
    toggleFlagged('val-total-miles', data.trip_sheet.flagged_fields.includes('total_miles'));

    // 4. Render Line Items Table
    const tbody = document.getElementById('line-items-body');
    tbody.innerHTML = '';
    
    if (data.trip_sheet.line_items.length === 0) {
        tbody.innerHTML = `<tr><td colspan="3" class="text-dim text-center">No route leg details extracted.</td></tr>`;
    } else {
        data.trip_sheet.line_items.forEach(item => {
            const tr = document.createElement('tr');
            tr.innerHTML = `
                <td class="font-mono">${item.date || 'N/A'}</td>
                <td>${item.location || 'N/A'}</td>
                <td class="text-right font-mono">${formatNum(item.miles)}</td>
            `;
            tbody.appendChild(tr);
        });
    }

    // 5. Render Validation Guardrails
    const alertBox = document.getElementById('validation-alert-box');
    const errorsList = document.getElementById('validation-errors-list');
    
    if (data.validation.errors.length === 0) {
        alertBox.style.display = 'none';
    } else {
        alertBox.style.display = 'flex';
        errorsList.innerHTML = '';
        data.validation.errors.forEach(err => {
            const li = document.createElement('li');
            li.innerText = err;
            errorsList.appendChild(li);
        });
    }

    // Update validation cards badges
    updateCardBadge('card-check-odo', data.validation.odometer_delta_check);
    updateCardBadge('card-check-sum', data.validation.line_item_sum_check);
    updateCardBadge('card-check-conf', data.validation.confidence_check);

    // 6. Render Exports code block
    renderExportCode();
}

function updateCardBadge(cardId, status) {
    const card = document.getElementById(cardId);
    const badge = card.querySelector('.status-check');
    if (status === 'pass') {
        badge.className = 'badge status-check pass';
        badge.innerHTML = '<i class="fa-solid fa-circle-check"></i> PASS';
    } else {
        badge.className = 'badge status-check fail';
        badge.innerHTML = '<i class="fa-solid fa-triangle-exclamation"></i> FAIL';
    }
}

function renderExportCode() {
    const codeBox = document.getElementById('export-code');
    const filenameLabel = document.getElementById('export-filename');
    const disclaimer = document.getElementById('export-disclaimer');

    if (!activeTrip) return;

    if (activeTrip.status !== 'validated') {
        filenameLabel.innerText = 'export_disabled.json';
        codeBox.innerText = JSON.stringify({
            error: "Export Blocked",
            reason: "This trip sheet is in the 'exception' queue and requires manual human review before export to downstream systems."
        }, null, 2);
        disclaimer.style.color = 'var(--red-accent)';
        return;
    }

    disclaimer.style.color = 'var(--on-surface-variant)';
    
    if (activeExportTab === 'tms') {
        filenameLabel.innerText = 'tms_dispatch.json';
        // Build mock TMS structure for visualization
        const tmsPayload = {
            export_type: "tms_dispatch",
            exported_at: new Date().toISOString(),
            trips: [{
                trip_id: activeTrip.ID || "450e2f9b-febf-4b81-aebe-bcbcdd6802f3",
                total_miles: activeTrip.trip_sheet.total_miles,
                route_segments: activeTrip.trip_sheet.line_items.map(li => {
                    const parts = (li.location || '').split(/(?:->|→| to )/);
                    return {
                        origin: parts[0] ? parts[0].trim() : "",
                        destination: parts[1] ? parts[1].trim() : "",
                        miles: li.miles,
                        date: li.date
                    };
                }),
                odometer: {
                    start: activeTrip.trip_sheet.odometer_open,
                    end: activeTrip.trip_sheet.odometer_close
                }
            }]
        };
        codeBox.innerText = JSON.stringify(tmsPayload, null, 2);
    } else {
        filenameLabel.innerText = 'accounting_payroll.json';
        const miles = activeTrip.trip_sheet.total_miles || 0;
        const rate = 0.55;
        const payrollPayload = {
            export_type: "accounting_payroll",
            exported_at: new Date().toISOString(),
            pay_items: [{
                trip_id: activeTrip.ID || "450e2f9b-febf-4b81-aebe-bcbcdd6802f3",
                date_range: {
                    start: activeTrip.trip_sheet.line_items[0]?.date || "",
                    end: activeTrip.trip_sheet.line_items[activeTrip.trip_sheet.line_items.length - 1]?.date || ""
                },
                total_miles: miles,
                billable_miles: miles,
                rate_per_mile: rate,
                total_pay: parseFloat((miles * rate).toFixed(2))
            }]
        };
        codeBox.innerText = JSON.stringify(payrollPayload, null, 2);
    }
}

// ==========================================================================
// Helpers
// ==========================================================================

function formatNum(val) {
    if (val === null || val === undefined) return 'null';
    return val.toLocaleString();
}

function toggleFlagged(elementId, isFlagged) {
    const el = document.getElementById(elementId);
    if (isFlagged) {
        el.style.color = 'var(--red-accent)';
        el.title = 'VLM flagged this field as illegible or missing';
    } else {
        el.style.color = '';
        el.title = '';
    }
}

function formatBytes(bytes, decimals = 2) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['Bytes', 'KB', 'MB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}
