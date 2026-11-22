(function attachTranscriptLibrary(global) {
    function createTranscriptLibrary({ list, searchInput, getCurrentUser, t, escapeHtml, escapeAttr, showToast, fallbackCopyTextToClipboard }) {
        const modal = global.document.getElementById('transcript-day-modal');
        const title = global.document.getElementById('transcript-day-title');
        const body = global.document.getElementById('transcript-day-body');
        const zone = global.Intl?.DateTimeFormat().resolvedOptions().timeZone || 'Asia/Seoul';
        let active = false, items = [], query = '', next = '', loading = false, error = false;
        let owner = null, version = 0, detailVersion = 0, request = null, detailRequest = null, timer = null, detailText = '', detailOpener = null;
        const pendingDeletes = new Set();
        const userID = () => getCurrentUser?.()?.id || '';
        const params = values => new URLSearchParams({ tz: zone, ...values });
        const timeLabel = value => new Date(value).toLocaleTimeString(undefined, { timeZone: zone, hour: '2-digit', minute: '2-digit' });
        const dateLabel = date => new Date(date+'T12:00:00Z').toLocaleDateString(undefined, { timeZone: 'UTC', year: 'numeric', month: 'long', day: 'numeric' });
        function closeDetail() {
            const wasOpen=modal?.classList.contains('active');
            detailVersion++; detailRequest?.abort(); detailText=''; modal?.classList.remove('active'); if(body)body.textContent='';
            if(wasOpen)detailOpener?.focus?.();
        }
        function checkOwner() {
            if(owner === userID())return;
            version++; request?.abort(); closeDetail(); items=[];query='';next='';owner=userID();pendingDeletes.clear();
        }
        function render() {
            if(!active || !list)return;
            checkOwner();
            if(searchInput)searchInput.placeholder=t('transcripts.search');
            list.innerHTML=items.map(item=>`
                <article class="saved-library-item">
                    <button type="button" class="saved-library-item-main transcript-open" data-transcript-open="${escapeAttr(item.date)}">
                        <div class="saved-library-item-title">${escapeHtml(dateLabel(item.date))}</div>
                        <div class="saved-library-item-preview">${escapeHtml(item.preview)}</div>
                        <div class="saved-library-item-meta">${escapeHtml(String(item.count))} ${escapeHtml(t('transcripts.records'))} · ${escapeHtml(timeLabel(item.startedAt))}–${escapeHtml(timeLabel(item.endedAt))}</div>
                    </button>
                    <button type="button" class="icon-btn" data-transcript-delete="${escapeAttr(item.date)}" title="${escapeAttr(t('transcripts.delete'))}" aria-label="${escapeAttr(t('transcripts.delete')+' '+dateLabel(item.date))}" ${pendingDeletes.has(item.date)?'disabled':''}>
                        <span class="material-icons-round">delete</span>
                    </button>
                </article>`).join('');
            if(loading || error || !items.length) list.innerHTML+=`<div class="saved-library-empty" role="status">${escapeHtml(t(loading?'transcripts.loading':error?'transcripts.failed':query?'library.emptyFiltered':'transcripts.empty'))}</div>`;
            if(!loading && (error || next))list.innerHTML+=`<button type="button" class="saved-turn-close-btn transcript-more" data-transcript-more>${escapeHtml(t(error?'transcripts.retry':'transcripts.more'))}</button>`;
        }
        async function load(append=false) {
            checkOwner(); if(!owner)return;
            const token=++version, account=owner;
            request?.abort(); request=new AbortController(); loading=true;error=false;
            if(!append){items=[];next='';}
            render();
            try {
                const response=await global.fetch('/api/stt/transcript-days?'+params({q:query,...(append && next?{before:next}:{})}),{credentials:'include',signal:request.signal});
                if(!response.ok)throw Error(`HTTP ${response.status}`);
                const data=await response.json();
                if(token!==version || account!==userID())return;
                items=append?[...items,...data.items]:data.items;next=data.next || '';
            }catch(e){if(token===version && account===userID() && e.name!=='AbortError')error=true;}
            finally{if(token===version){loading=false;render();}}
        }
        function search(value) {
            query=String(value || '');clearTimeout(timer);version++;request?.abort();
            items=[];next='';loading=true;error=false;render();
            timer=global.setTimeout(()=>load(),250);
        }
        async function openDay(date) {
            checkOwner(); const account=owner, token=++detailVersion;
            detailOpener=global.document.activeElement;
            detailRequest?.abort();detailRequest=new AbortController();detailText='';
            if(title)title.textContent=dateLabel(date);
            if(body)body.textContent=t('transcripts.loading');
            modal?.classList.add('active');
            modal?.querySelector?.('[data-transcript-close]')?.focus();
            try {
                const response=await global.fetch('/api/stt/transcripts?'+params({date}),{credentials:'include',signal:detailRequest.signal});
                if(!response.ok)throw Error(`HTTP ${response.status}`);
                const data=await response.json();
                if(token!==detailVersion || account!==userID())return;
                detailText=data.entries.map(entry=>`[${timeLabel(entry.startedAt)}] ${entry.text}`).join('\n\n');
                if(body)body.textContent=detailText || t('transcripts.empty');
            }catch(e){if(token===detailVersion && account===userID() && e.name!=='AbortError' && body)body.textContent=t('transcripts.failed');}
        }
        async function deleteDay(date) {
            checkOwner();const item=items.find(item=>item.date===date),account=owner;
            if(!item || pendingDeletes.has(date))return;
            if(!global.confirm(`${dateLabel(date)} · ${item.count} ${t('transcripts.records')}\n${t('transcripts.deleteConfirm')}`))return;
            pendingDeletes.add(date);render();
            try {
                const response=await global.fetch('/api/stt/transcripts?'+params({date}),{method:'DELETE',credentials:'include'});
                if(!response.ok)throw Error(`HTTP ${response.status}`);
                if(account!==userID())return;
                closeDetail();await load();showToast(t('transcripts.deleted'));
            }catch(e){if(account===userID())showToast(t('transcripts.deleteFailed'),true);}
            finally{pendingDeletes.delete(date);render();}
        }
        list?.addEventListener('click',event=>{
            if(!active)return;
            const remove=event.target.closest('[data-transcript-delete]');
            const open=event.target.closest('[data-transcript-open]');
            if(remove){event.stopPropagation();deleteDay(remove.dataset.transcriptDelete);}
            else if(open)openDay(open.dataset.transcriptOpen);
            else if(event.target.closest('[data-transcript-more]'))load(items.length>0 && !!next);
        });
        modal?.addEventListener('click',async event=>{
            if(event.target===modal || event.target.closest('[data-transcript-close]'))closeDetail();
            if(event.target.closest('[data-transcript-copy]') && detailText){
                try{await global.navigator.clipboard.writeText(detailText);showToast(t('clipboard.copied'));}
                catch{fallbackCopyTextToClipboard(detailText);}
            }
        });
        global.document.addEventListener('keydown',event=>{
            if(!modal?.classList.contains('active'))return;
            if(event.key==='Escape'){event.preventDefault();event.stopImmediatePropagation();closeDetail();}
            if(event.key==='Tab'){
                const buttons=modal.querySelectorAll('button:not([disabled])');
                const first=buttons[0],last=buttons[buttons.length-1];
                if(event.shiftKey && global.document.activeElement===first){event.preventDefault();last?.focus();}
                else if(!event.shiftKey && global.document.activeElement===last){event.preventDefault();first?.focus();}
            }
        });
        return {render,search,getQuery:()=>query,isActive:()=>active,
            setActive(value){checkOwner();active=value;if(active){render();load();}else{clearTimeout(timer);version++;request?.abort();closeDetail();}},
            close(){clearTimeout(timer);version++;request?.abort();closeDetail();},load};
    }
    global.DKSTTranscriptLibrary={createTranscriptLibrary};
})(window);
