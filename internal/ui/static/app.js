let sessionId = '';
let currentApprovalId = '';

document.getElementById('newSession').addEventListener('click', async () => {
    const res = await fetch('/v1/session', { method: 'POST' });
    const data = await res.json();
    sessionId = data.session_id;
    document.getElementById('sessionId').textContent = sessionId;
    document.getElementById('approveBtn').disabled = true;
    document.getElementById('denyBtn').disabled = true;
    document.getElementById('approvalInfo').textContent = 'No pending approval.';
});

document.getElementById('stageBtn').addEventListener('click', async () => {
    const slot = document.getElementById('slotInput').value;
    const text = document.getElementById('textInput').value;
    const res = await fetch('/v1/stage', {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + sessionId, 'Content-Type': 'application/json' },
        body: JSON.stringify({ slot: parseInt(slot), text })
    });
    const data = await res.json();
    document.getElementById('stageResult').textContent = JSON.stringify(data, null, 2);
});

document.getElementById('proposeBtn').addEventListener('click', async () => {
    const seqInput = document.getElementById('seqInput').value;
    let seq;
    try {
        seq = JSON.parse(seqInput);
        if (!Array.isArray(seq)) throw new Error();
    } catch {
        document.getElementById('proposeResult').textContent = 'Invalid sequence format. Use [num, num, ...]';
        return;
    }
    const payload = { v: 1, seq };
    const res = await fetch('/v1/propose', {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + sessionId, 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
    });
    const data = await res.json();
    document.getElementById('proposeResult').textContent = JSON.stringify(data, null, 2);
    if (data.status === 'CONFIRMATION_REQUIRED') {
        currentApprovalId = data.approval_id;
        document.getElementById('approvalInfo').textContent = `Approval required (expires ${data.expires_at})`;
        document.getElementById('approveBtn').disabled = false;
        document.getElementById('denyBtn').disabled = false;
    }
});

document.getElementById('approveBtn').addEventListener('click', async () => {
    await sendApproval('approve');
});

document.getElementById('denyBtn').addEventListener('click', async () => {
    await sendApproval('deny');
});

async function sendApproval(decision) {
    const res = await fetch('/v1/approve', {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + sessionId, 'Content-Type': 'application/json' },
        body: JSON.stringify({ approval_id: currentApprovalId, decision })
    });
    const data = await res.json();
    alert(`Approval ${decision} result: ${data.status}`);
    document.getElementById('approveBtn').disabled = true;
    document.getElementById('denyBtn').disabled = true;
    document.getElementById('approvalInfo').textContent = 'No pending approval.';
    currentApprovalId = '';
}

document.getElementById('refreshDrafts').addEventListener('click', async () => {
    const res = await fetch('/v1/drafts', { headers: { 'Authorization': 'Bearer ' + sessionId } });
    const data = await res.json();
    const ul = document.getElementById('draftsList');
    ul.innerHTML = '';
    if (data.drafts) {
        data.drafts.forEach(d => {
            const li = document.createElement('li');
            li.textContent = d;
            ul.appendChild(li);
        });
    }
});
