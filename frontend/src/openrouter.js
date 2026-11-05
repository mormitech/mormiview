// The chat through OpenRouter (openrouter.ai): its chat completions API (the OpenAI form), streamed, with the user's OpenRouter key.
// The tools are the same picture tools as with the Claude API; a picture a tool answers with goes to the model as a user message right
// after the tool results (a tool result of this API holds text only). See chat-api.js for the panel's side.

import {callTool, short} from './chat-api.js';

const API = 'https://openrouter.ai/api/v1';
export const OR_DEFAULT = 'anthropic/claude-opus-5.5';

let modelList = null; // the promise of OpenRouter's models that can use tools and see pictures: [{id, name, reasoning, maxOut}]

// orModels is that list (public: it needs no key), loaded once.
export function orModels() {
  modelList ??= fetch(`${API}/models`).then((r) => {
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
    return r.json();
  }).then((j) => (j.data || [])
    .filter((m) => (m.supported_parameters || []).includes('tools') && (m.architecture?.input_modalities || []).includes('image') && !m.id.endsWith(':batch'))
    .map((m) => ({id: m.id, name: m.name || m.id, reasoning: (m.supported_parameters || []).includes('reasoning'), maxOut: m.top_provider?.max_completion_tokens || 0}))
    .sort((a, b) => (b.id === OR_DEFAULT) - (a.id === OR_DEFAULT) || (b.id.startsWith('anthropic/')) - (a.id.startsWith('anthropic/')) || a.name.localeCompare(b.name)))
    .catch((e) => { modelList = null; throw e; });
  return modelList;
}

let emit = () => {};
const toolDefs = (list) => list.map((t) => ({type: 'function', function: {name: t.name, description: t.description, parameters: t.inputSchema}}));

// An error of the API, with what it said.
class ORError extends Error {
  constructor(status, message) { super(message); this.status = status; }
}

export function orError(e) {
  if (!(e instanceof ORError)) return e instanceof TypeError ? 'OpenRouter could not be reached (offline?).' : String(e?.message || e);
  if (e.status === 401) return 'The OpenRouter key was not accepted. Set another one in AI → AI Provider and Key….';
  if (e.status === 402) return 'The OpenRouter account has no credits left for this: add credits at openrouter.ai, or choose a cheaper model.';
  if (e.status === 429) return 'Too many requests for this OpenRouter key just now; wait a little and try again.';
  return `OpenRouter answered: ${e.message}`;
}

// One request, streamed: what the model writes goes to the panel as it comes; the answer is the assistant message and why it stopped.
async function request(key, body, signal) {
  const r = await fetch(`${API}/chat/completions`, {
    method: 'POST', signal,
    headers: {'Authorization': `Bearer ${key}`, 'Content-Type': 'application/json', 'HTTP-Referer': location.origin, 'X-Title': 'mormiView'},
    body: JSON.stringify({...body, stream: true}),
  });
  if (!r.ok) {
    let m = r.statusText || `HTTP ${r.status}`;
    try { const j = await r.json(); m = j.error?.metadata?.raw || j.error?.message || m; } catch { /* not JSON */ }
    throw new ORError(r.status, typeof m === 'string' ? m : JSON.stringify(m));
  }
  const msg = {role: 'assistant', content: ''}, calls = [], reasoning = [];
  let finish = null;
  const reader = r.body.pipeThrough(new TextDecoderStream()).getReader();
  let buf = '';
  const line = (l) => {
    if (!l.startsWith('data:')) return; // (": OPENROUTER PROCESSING" keeps the line open)
    const d = l.slice(5).trim();
    if (!d || d === '[DONE]') return;
    const j = JSON.parse(d);
    if (j.error) throw new ORError(j.error.code || 500, j.error.message || 'the answer broke off');
    const c = j.choices?.[0];
    if (!c) return;
    const dl = c.delta || {};
    if (dl.content) { msg.content += dl.content; emit({kind: 'delta', text: dl.content}); }
    for (const tc of dl.tool_calls || []) {
      const i = tc.index ?? calls.length;
      calls[i] ??= {id: '', type: 'function', function: {name: '', arguments: ''}};
      if (tc.id) calls[i].id = tc.id;
      if (tc.function?.name) calls[i].function.name += tc.function.name;
      if (tc.function?.arguments) calls[i].function.arguments += tc.function.arguments;
    }
    // The model's reasoning, kept to be sent back (a model that thinks between tool calls needs its reasoning of this turn): the pieces of one
    // block come with the same index and are joined.
    for (const rd of dl.reasoning_details || []) {
      const i = rd.index ?? reasoning.length;
      const cur = reasoning[i];
      if (!cur) { reasoning[i] = {...rd}; continue; }
      for (const f of ['text', 'summary', 'data']) if (typeof rd[f] === 'string') cur[f] = (cur[f] || '') + rd[f];
      for (const [f, val] of Object.entries(rd)) if (!['text', 'summary', 'data', 'index'].includes(f) && val != null) cur[f] = val;
    }
    if (c.finish_reason) finish = c.finish_reason;
  };
  for (;;) {
    const {value, done} = await reader.read();
    if (done) break;
    buf += value;
    let n;
    while ((n = buf.indexOf('\n')) >= 0) { line(buf.slice(0, n).trim()); buf = buf.slice(n + 1); }
  }
  if (buf.trim()) line(buf.trim());
  const tc = calls.filter(Boolean);
  if (tc.length) msg.tool_calls = tc.map((c, i) => ({...c, id: c.id || `call_${Date.now()}_${i}`}));
  const rs = reasoning.filter(Boolean);
  if (rs.length) msg.reasoning_details = rs;
  if (!msg.content) msg.content = null;
  return {msg, finish};
}

// The answers to a turn's tool calls: a tool message each (text), then one user message with the pictures they answered with.
async function answer(calls, signal) {
  const out = [], pics = [];
  for (const call of calls) {
    if (signal.aborted) break;
    let input = null;
    try { input = JSON.parse(call.function.arguments || '{}'); } catch { /* below */ }
    emit({kind: 'tool', name: call.function.name, input: short(input ?? call.function.arguments)});
    if (!input || typeof input !== 'object' || Array.isArray(input)) {
      out.push({role: 'tool', tool_call_id: call.id, content: 'INVALID_JSON: the input of the tool was not a JSON object; send it again.'});
      emit({kind: 'toolerror', text: `${call.function.name}: the input was not complete`});
      continue;
    }
    const r = await callTool(call.function.name, input);
    if (r.error) {
      out.push({role: 'tool', tool_call_id: call.id, content: `Error: ${r.error}`});
      emit({kind: 'toolerror', text: `${call.function.name}: ${r.error}`});
      continue;
    }
    const texts = [];
    for (const c of r.content || []) {
      if (c.type === 'text' && c.text) texts.push(c.text);
      else if (c.type === 'image') { pics.push({type: 'image_url', image_url: {url: `data:${c.mimeType || 'image/png'};base64,${c.data}`}}); texts.push('(the picture is in the next message)'); }
    }
    out.push({role: 'tool', tool_call_id: call.id, content: texts.join('\n') || 'done'});
  }
  if (pics.length) out.push({role: 'user', content: [{type: 'text', text: 'The pictures the tools answered with:'}, ...pics]});
  return out;
}

// A conversation must not end with tool calls that have no answers (after Stop): they are answered as stopped.
function closeOpenCalls(messages) {
  const last = messages.at(-1);
  if (last?.role !== 'assistant' || !last.tool_calls?.length) return;
  for (const c of last.tool_calls) messages.push({role: 'tool', tool_call_id: c.id, content: 'Stopped by the user.'});
}

// orRun sends text and goes on while the model calls tools. It changes messages (the conversation) and says whether it was stopped.
export async function orRun({key, model, messages, text, signal, maxTurns, tools: list, system: prompt, emit: fn}) {
  emit = fn;
  closeOpenCalls(messages);
  messages.push({role: 'user', content: text});
  const info = await orModels().then((l) => l.find((m) => m.id === model)).catch(() => null);
  const anthropic = model.startsWith('anthropic/');
  let plain = false; // plain: the request without the optional parts (reasoning, caching), after OpenRouter refused it once
  try {
    for (let turn = 0; turn < maxTurns; turn++) {
      emit({kind: 'start'});
      const system = {role: 'system', content: [{type: 'text', text: prompt, ...(anthropic && !plain ? {cache_control: {type: 'ephemeral'}} : {})}]};
      const body = {
        model,
        messages: [system, ...(plain ? messages.map(({reasoning_details, ...m}) => m) : messages)],
        tools: toolDefs(list),
        max_tokens: Math.min(64000, info?.maxOut || 32000),
      };
      if (!plain && (info ? info.reasoning : true)) body.reasoning = {effort: 'high'};
      let res;
      try {
        res = await request(key, body, signal);
      } catch (e) {
        if (!plain && e instanceof ORError && e.status === 400 && !signal.aborted) { console.warn('mormiView chat: retrying without optional parts:', e.message); plain = true; turn--; continue; }
        throw e;
      }
      const {msg, finish} = res;
      messages.push(msg);
      if (finish === 'length') { closeOpenCalls(messages); emit({kind: 'done', error: true, text: 'The answer was too long and was cut off.'}); return false; }
      if (finish === 'content_filter') { closeOpenCalls(messages); emit({kind: 'done', error: true, text: 'The model declined to answer this.'}); return false; }
      if (!msg.tool_calls?.length) { emit({kind: 'done', error: false, text: ''}); return false; }
      const results = await answer(msg.tool_calls, signal);
      if (signal.aborted) {
        const answered = new Set(results.filter((m) => m.role === 'tool').map((m) => m.tool_call_id));
        messages.push(...results.filter((m) => m.role === 'tool'));
        for (const c of msg.tool_calls) if (!answered.has(c.id)) messages.push({role: 'tool', tool_call_id: c.id, content: 'Stopped by the user.'});
        return true;
      }
      messages.push(...results);
      if (turn === maxTurns - 1) emit({kind: 'done', error: true, text: 'Stopped after many steps; say "go on" to continue.'});
    }
  } catch (e) {
    closeOpenCalls(messages);
    throw e;
  }
  return false;
}
