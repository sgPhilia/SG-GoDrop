(function() {
    const dropZone = document.getElementById('drop-zone');
    const fileInput = document.getElementById('file-input');
    const chooseBtn = document.getElementById('choose-btn');
    const container = document.getElementById('transfers-container');
    const lanUrlSpan = document.getElementById('lan-url');
    const copyBtn = document.getElementById('copy-btn');
    const copyFeedback = document.getElementById('copy-feedback');

    let pollInterval = null;
    let currentUrl = '';

    async function fetchInfo() {
        try {
            const resp = await fetch('/api/info');
            if (!resp.ok) throw new Error('Failed to fetch info');
            const data = await resp.json();
            currentUrl = data.url || '';
            if (lanUrlSpan) {
                lanUrlSpan.textContent = currentUrl;
            }
        } catch (err) {
            console.error('Error fetching info:', err);
        }
    }

    if (copyBtn) {
        copyBtn.addEventListener('click', async () => {
            if (!currentUrl) {
                copyFeedback.textContent = 'No URL available';
                return;
            }
            try {
                await navigator.clipboard.writeText(currentUrl);
                copyFeedback.textContent = 'Copied!';
                setTimeout(() => { copyFeedback.textContent = ''; }, 3000);
            } catch (err) {
                const input = document.createElement('input');
                input.value = currentUrl;
                document.body.appendChild(input);
                input.select();
                document.execCommand('copy');
                document.body.removeChild(input);
                copyFeedback.textContent = 'Copied!';
                setTimeout(() => { copyFeedback.textContent = ''; }, 3000);
            }
        });
    }

    async function fetchTransfers() {
        try {
            const resp = await fetch('/api/transfers');
            if (!resp.ok) throw new Error('Failed to fetch transfers');
            const data = await resp.json();
            renderTransfers(data.transfers);
        } catch (err) {
            console.error('Error fetching transfers:', err);
        }
    }

    function renderTransfers(transfers) {
        if (!transfers || transfers.length === 0) {
            container.innerHTML = '<p style="color:#94a3b8;">No transfers yet.</p>';
            return;
        }
        let html = '';
        for (const t of transfers) {
            const statusClass = t.status;
            const progress = t.size > 0 ? (t.progress / t.size * 100) : 0;
            let actions = '';
            if (t.status === 'completed') {
                actions = `<a href="/api/transfers/${t.id}/download" class="btn-small">Download</a>
                           <button class="btn-small danger" data-id="${t.id}" data-action="delete">Delete</button>`;
            } else if (t.status === 'uploading' || t.status === 'pending') {
                actions = `<span style="font-size:0.8rem;color:#64748b;">In progress...</span>`;
            } else {
                actions = `<button class="btn-small danger" data-id="${t.id}" data-action="delete">Delete</button>`;
            }
            html += `
                <div class="transfer-item" data-id="${t.id}">
                    <div class="transfer-info">
                        <div class="transfer-name">${escapeHtml(t.name)}</div>
                        <div class="transfer-meta">${formatFileSize(t.size)} · ${t.status}</div>
                        <div class="transfer-progress">
                            <div class="transfer-progress-bar" style="width:${Math.min(progress,100)}%"></div>
                        </div>
                    </div>
                    <div class="transfer-status ${statusClass}">${t.status}</div>
                    <div class="transfer-actions">${actions}</div>
                </div>
            `;
        }
        container.innerHTML = html;

        document.querySelectorAll('[data-action="delete"]').forEach(btn => {
            btn.addEventListener('click', async (e) => {
                const id = btn.dataset.id;
                if (!confirm('Delete this transfer?')) return;
                try {
                    const resp = await fetch(`/api/transfers/${id}`, { method: 'DELETE' });
                    if (!resp.ok) throw new Error('Delete failed');
                    fetchTransfers();
                } catch (err) {
                    alert('Failed to delete: ' + err.message);
                }
            });
        });
    }

    function formatFileSize(bytes) {
        if (bytes === 0) return '0 B';
        const units = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
    }

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    async function uploadFiles(files) {
        for (const file of files) {
            const formData = new FormData();
            formData.append('file', file);
            try {
                const resp = await fetch('/api/upload', {
                    method: 'POST',
                    body: formData,
                });
                if (!resp.ok) {
                    const text = await resp.text();
                    throw new Error(text || 'Upload failed');
                }
                const data = await resp.json();
                console.log('Uploaded:', data);
                fetchTransfers();
            } catch (err) {
                alert(`Upload failed for ${file.name}: ${err.message}`);
            }
        }
    }

    dropZone.addEventListener('dragover', (e) => {
        e.preventDefault();
        dropZone.classList.add('dragover');
    });

    dropZone.addEventListener('dragleave', () => {
        dropZone.classList.remove('dragover');
    });

    dropZone.addEventListener('drop', (e) => {
        e.preventDefault();
        dropZone.classList.remove('dragover');
        if (e.dataTransfer.files.length) {
            uploadFiles(e.dataTransfer.files);
        }
    });

    dropZone.addEventListener('click', () => {
        fileInput.click();
    });

    chooseBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        fileInput.click();
    });

    fileInput.addEventListener('change', () => {
        if (fileInput.files.length) {
            uploadFiles(fileInput.files);
            fileInput.value = '';
        }
    });

    fetchInfo();
    fetchTransfers();
    pollInterval = setInterval(fetchTransfers, 2000);

    window.addEventListener('beforeunload', () => {
        if (pollInterval) clearInterval(pollInterval);
    });
})();