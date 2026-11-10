// The chat panel's own conversation with a model API, from the page, with the user's own API key: the Claude API (api.anthropic.com, with
// the Anthropic SDK) or OpenRouter (openrouter.ai, its chat completions API: openrouter.js), chosen in AI → AI Provider and Key….
// The tools are the very same picture tools Claude Code gets on the desktop (mcp.go), called through the Go methods of chat_api.go.
// The web version always chats this way (a page cannot start Claude Code); the desktop app does when one of these providers is chosen
// instead of Claude Code. The panel (chat.js) is the same for both: it gets the same "chat" events (through onChatEvent).

import {ChatAPIConfig, SetChatAPIConfig, ChatTools, ChatPrompt, ChatTool} from '../wailsjs/go/main/App';
import {formDialog} from './dialogs.js';
import {OR_DEFAULT, orModels, orRun, orError} from './openrouter.js';

const MODELS = [
  ['claude-opus-5-5', 'Claude Opus 5.5 (best for retouching)'],
  ['claude-sonnet-5-5', 'Claude Sonnet 5.5 (faster, cheaper)'],
  ['claude-haiku-5-5', 'Claude Haiku 5.5 (fastest, cheapest)'],
];
const MAX_TURNS = 80;

// The settings (ai.json) and the remembered keys (keys.json) are kept by the Go side, in the settings folder (in the web version: in the
// browser's storage, with the other settings). A key that is not to be remembered is kept here only, until the page is closed.
let cfg = null;
const memKeys = {anthropic: '', openrouter: ''};
async function config() {
  if (cfg) return cfg;
  cfg = await ChatAPIConfig();
  try { // (keys the first web version kept in the browser's localStorage move to the settings)
    const old = {key: localStorage.getItem('mormiview.apiKey'), orKey: localStorage.getItem('mormiview.openrouterKey'), p: localStorage.getItem('mormiview.provider'), m: localStorage.getItem('mormiview.model'), om: localStorage.getItem('mormiview.openrouterModel')};
    if (Object.values(old).some(Boolean)) {
      cfg = {...cfg, provider: old.p || cfg.provider, anthropicModel: old.m || cfg.anthropicModel, openRouterModel: old.om || cfg.openRouterModel, anthropicKey: old.key || cfg.anthropicKey, openRouterKey: old.orKey || cfg.openRouterKey};
      await SetChatAPIConfig(cfg.provider || 'anthropic', cfg.anthropicModel || '', cfg.openRouterModel || '', cfg.anthropicKey || '', cfg.openRouterKey || '');
      for (const k of ['apiKey', 'openrouterKey', 'provider', 'model', 'openrouterModel']) { localStorage.removeItem(`mormiview.${k}`); sessionStorage.removeItem(`mormiview.${k}`); }
    }
  } catch { /* storage off */ }
  return cfg;
}

let emit = () => {};
// onChatEvent gives the panel's handler of the "chat" events.
export function onChatEvent(fn) { emit = fn; }

let Anthropic = null; // the SDK, loaded with the first message (the page does not need it until then)
const sdk = async () => (Anthropic ??= (await import('@anthropic-ai/sdk')).default);
let messages = []; // the conversation with the Claude API (its own format)
let orMessages = []; // the conversation through OpenRouter (the chat completions format)
let busy = null; // {abort: AbortController}

export const short = (v, n = 160) => { const s = JSON.stringify(v ?? {}); return s.length > n ? `${s.slice(0, n - 1)}…` : s; };
// provider: '' is Claude Code (the desktop's own way, chat.go), 'anthropic' the Claude API, 'openrouter' OpenRouter.
const provider = () => (cfg.provider === 'openrouter' || cfg.provider === 'anthropic' ? cfg.provider : cfg.desktop ? '' : 'anthropic');
const model = () => cfg.anthropicModel || MODELS[0][0];
const orModel = () => cfg.openRouterModel || OR_DEFAULT;
const keyOf = (p) => (p === 'openrouter' ? memKeys.openrouter || cfg.openRouterKey : memKeys.anthropic || cfg.anthropicKey) || '';
const activeKey = () => keyOf(provider());

// usesAPI says whether the chat goes to a model API from the page (rather than to Claude Code).
export async function usesAPI() { await config(); return provider() !== ''; }

// The picture tools and the system prompt, from the Go side (once).
let toolList = null, prompt = '';
export async function toolInfo() {
  if (!toolList) [toolList, prompt] = await Promise.all([ChatTools().then((j) => JSON.parse(j)), ChatPrompt()]);
  return {tools: toolList, prompt};
}

function tools() {
  const out = toolList.map((t) => ({name: t.name, description: t.description, input_schema: t.inputSchema, eager_input_streaming: true}));
  if (!cfg.noWeb && model() !== 'claude-haiku-5-5') out.push({type: 'web_search_20260209', name: 'web_search', max_uses: 5});
  out[out.length - 1] = {...out[out.length - 1], cache_control: {type: 'ephemeral'}};
  return out;
}

// callTool runs a picture tool: {content} (MCP content: text and images) or {error}.
export async function callTool(name, input) {
  try { return {content: JSON.parse(await ChatTool(name, JSON.stringify(input)))}; } catch (e) { return {error: String(e?.message || e)}; }
}

// The answer of a picture tool (MCP content: text and PNG images) as the content of a tool_result.
function resultContent(items) {
  const out = [];
  for (const c of items || []) {
    if (c.type === 'image') out.push({type: 'image', source: {type: 'base64', media_type: c.mimeType || 'image/png', data: c.data}});
    else if (c.type === 'text') out.push({type: 'text', text: c.text || ' '});
  }
  return out.length ? out : [{type: 'text', text: 'done'}];
}

// A conversation must not end with tool calls that have no results (after Stop): they are answered as stopped.
function closeOpenCalls() {
  const last = messages.at(-1);
  if (last?.role !== 'assistant' || !Array.isArray(last.content)) return;
  const calls = last.content.filter((b) => b.type === 'tool_use');
  if (calls.length) messages.push({role: 'user', content: calls.map((b) => ({type: 'tool_result', tool_use_id: b.id, is_error: true, content: 'Stopped by the user.'}))});
}

function addUser(text) {
  closeOpenCalls();
  const last = messages.at(-1);
  if (last?.role === 'user') { // (an answer that was stopped before it began)
    if (typeof last.content === 'string') last.content = [{type: 'text', text: last.content}];
    last.content.push({type: 'text', text});
  } else messages.push({role: 'user', content: text});
}

async function run(text) {
  await Promise.all([sdk(), toolInfo()]);
  cfg = {...cfg, noWeb: (await ChatAPIConfig()).noWeb}; // (AI → Web search and fetch may have changed)
  const key = activeKey();
  const client = new Anthropic({apiKey: key, dangerouslyAllowBrowser: true});
  const ctl = new AbortController();
  busy = {abort: ctl};
  addUser(text);
  let stopped = false, plain = false; // plain: the request without the optional features, after the API refused them once
  try {
    for (let turn = 0; turn < MAX_TURNS; turn++) {
      emit({kind: 'start'});
      const m = model();
      const params = {
        model: m,
        max_tokens: 64000,
        system: [{type: 'text', text: prompt, cache_control: {type: 'ephemeral'}}],
        tools: plain ? tools().map(({eager_input_streaming, ...t}) => t).filter((t) => !t.type) : tools(),
        messages,
        thinking: {type: 'adaptive'},
        output_config: {effort: 'high'},
      };
      if (m !== 'claude-haiku-5-5' && !plain) { params.betas = ['server-side-fallback-2026-07-01']; params.fallbacks = 'default'; }
      let msg;
      try {
        const stream = client.beta.messages.stream(params, {signal: ctl.signal});
        for await (const ev of stream) {
          if (ev.type === 'content_block_delta' && ev.delta.type === 'text_delta' && ev.delta.text) emit({kind: 'delta', text: ev.delta.text});
          if (ev.type === 'content_block_start' && ev.content_block.type === 'server_tool_use') emit({kind: 'tool', name: ev.content_block.name, input: ''});
        }
        msg = await stream.finalMessage();
      } catch (e) {
        // A request the API refuses as such (an optional feature this key or model does not have): once more with only what the chat needs.
        if (!plain && e instanceof Anthropic.BadRequestError && !ctl.signal.aborted) { console.warn('mormiView chat: retrying without optional features:', e.message); plain = true; turn--; continue; }
        throw e;
      }
      messages.push({role: 'assistant', content: msg.content});
      if (msg.stop_reason === 'refusal') { emit({kind: 'done', error: true, text: msg.stop_details?.explanation || 'Claude declined to answer this.'}); break; }
      if (msg.stop_reason === 'pause_turn') continue;
      const calls = msg.content.filter((b) => b.type === 'tool_use');
      if (msg.stop_reason === 'max_tokens') {
        closeOpenCalls();
        emit({kind: 'done', error: true, text: 'The answer was too long and was cut off.'});
        break;
      }
      if (msg.stop_reason !== 'tool_use' || !calls.length) { emit({kind: 'done', error: false, text: ''}); break; }
      const results = [];
      for (const call of calls) {
        if (ctl.signal.aborted) break;
        emit({kind: 'tool', name: call.name, input: short(call.input)});
        if (!call.input || typeof call.input !== 'object' || Array.isArray(call.input)) {
          results.push({type: 'tool_result', tool_use_id: call.id, is_error: true, content: 'INVALID_JSON: the input of the tool was not a JSON object; send it again.'});
          emit({kind: 'toolerror', text: `${call.name}: the input was not complete`});
          continue;
        }
        const r = await callTool(call.name, call.input);
        if (r.error) {
          results.push({type: 'tool_result', tool_use_id: call.id, is_error: true, content: r.error});
          emit({kind: 'toolerror', text: `${call.name}: ${r.error}`});
        } else results.push({type: 'tool_result', tool_use_id: call.id, content: resultContent(r.content)});
      }
      if (ctl.signal.aborted) { stopped = true; break; }
      messages.push({role: 'user', content: results});
      if (turn === MAX_TURNS - 1) emit({kind: 'done', error: true, text: 'Stopped after many steps; say "go on" to continue.'});
    }
  } catch (e) {
    if (ctl.signal.aborted || e?.name === 'AbortError' || e instanceof Anthropic.APIUserAbortError) stopped = true;
    else emit({kind: 'error', text: apiError(e)});
    closeOpenCalls();
  } finally {
    busy = null;
    emit({kind: 'end', stopped});
  }
}

async function runOpenRouter(text) {
  const ctl = new AbortController();
  busy = {abort: ctl};
  let stopped = false;
  try {
    const {tools: list, prompt: system} = await toolInfo();
    stopped = await orRun({key: keyOf('openrouter'), model: orModel(), messages: orMessages, text, signal: ctl.signal, maxTurns: MAX_TURNS, tools: list, system, emit});
  } catch (e) {
    if (ctl.signal.aborted || e?.name === 'AbortError') stopped = true;
    else emit({kind: 'error', text: orError(e)});
  } finally {
    busy = null;
    emit({kind: 'end', stopped});
  }
}

function apiError(e) {
  if (!Anthropic) return String(e?.message || e);
  if (e instanceof Anthropic.AuthenticationError) return 'The API key was not accepted. Set another one with the pencil next to the model, or in AI → AI Provider and Key….';
  if (e instanceof Anthropic.PermissionDeniedError) return `This API key may not do that: ${e.message}`;
  if (e instanceof Anthropic.RateLimitError) return 'Too many requests for this API key just now; wait a little and try again.';
  if (e instanceof Anthropic.APIConnectionError) return 'The Claude API could not be reached (offline?).';
  if (e instanceof Anthropic.APIError) return `The Claude API answered: ${e.message}`;
  return String(e?.message || e);
}

// askProvider is AI → AI Provider and Key… (and the pencil in the panel): who answers, with which key and model. It says whether
// something was chosen, and whether the provider changed (then the panel starts a new conversation: the providers keep it differently).
export async function askProvider() {
  await config();
  const esc = (v) => String(v ?? '').replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;');
  const was = provider(), hasAny = !!(keyOf('anthropic') || keyOf('openrouter'));
  const keep = !(memKeys.anthropic || memKeys.openrouter); // (remembered unless a key was given without remembering it)
  const opt = (v, n) => `<option value="${v}"${was === v ? ' selected' : ''}>${n}</option>`;
  const v = await formDialog('AI provider and API key', `
    <div class="mv-row"><label for="mv-prov">Provider</label><select id="mv-prov">
      ${cfg.desktop ? opt('', 'Claude Code (your Claude subscription)') : ''}${opt('anthropic', 'Claude API (Anthropic)')}${opt('openrouter', 'OpenRouter')}</select></div>
    <div class="mv-prov" data-p="">
      <p class="mv-msg">The chat runs your own Claude Code with your Claude subscription (Pro or Max): no API key, nothing billed by usage. The panel signs you in to Claude Code when it needs to.</p>
    </div>
    <div class="mv-prov" data-p="anthropic">
      <p class="mv-msg">The chat talks to the Claude API with your own API key (from <a href="https://platform.claude.com/settings/keys" target="_blank" rel="noopener">platform.claude.com</a>), and is billed by usage to that key. The key stays ${cfg.desktop ? 'on this computer' : 'in this browser'} and is sent only to api.anthropic.com.</p>
      <div class="mv-row"><label for="mv-key">API key</label><input id="mv-key" type="password" spellcheck="false" autocomplete="off" placeholder="sk-ant-…" value="${esc(keyOf('anthropic'))}"${was === 'anthropic' ? ' autofocus' : ''}></div>
      <div class="mv-row"><label for="mv-model">Model</label><select id="mv-model">${MODELS.map(([id, n]) => `<option value="${id}"${id === model() ? ' selected' : ''}>${n}</option>`).join('')}</select></div>
    </div>
    <div class="mv-prov" data-p="openrouter">
      <p class="mv-msg">The chat talks to OpenRouter with your own OpenRouter key (from <a href="https://openrouter.ai/settings/keys" target="_blank" rel="noopener">openrouter.ai</a>), and is billed to your OpenRouter credits. The key stays ${cfg.desktop ? 'on this computer' : 'in this browser'} and is sent only to openrouter.ai, which passes the chat on to the model's provider.</p>
      <div class="mv-row"><label for="mv-orkey">API key</label><input id="mv-orkey" type="password" spellcheck="false" autocomplete="off" placeholder="sk-or-…" value="${esc(keyOf('openrouter'))}"${was === 'openrouter' ? ' autofocus' : ''}></div>
      <div class="mv-row"><label for="mv-ormodel">Model</label><input id="mv-ormodel" list="mv-ormodels" spellcheck="false" autocomplete="off" value="${esc(orModel())}"></div>
      <datalist id="mv-ormodels"></datalist>
      <p class="mv-note" id="mv-ornote">Any OpenRouter model that can use tools and see pictures; ${OR_DEFAULT} is the best for retouching.</p>
    </div>
    <label class="mv-check mv-keys"><input type="checkbox" id="mv-keep"${keep ? ' checked' : ''}> Remember the keys ${cfg.desktop ? 'on this computer' : 'in this browser'}</label>
    ${hasAny ? '<p class="mv-note mv-keys"><a href="#" id="mv-forget">Forget the keys</a></p>' : ''}`, (box) => {
    const p = box.querySelector('#mv-prov').value;
    const k = box.querySelector('#mv-key').value.trim(), ork = box.querySelector('#mv-orkey').value.trim();
    if (p === 'anthropic' && !k) return 'Paste your Claude API key.';
    if (p === 'openrouter' && !ork) return 'Paste your OpenRouter API key.';
    const om = box.querySelector('#mv-ormodel').value.trim() || OR_DEFAULT;
    if (p === 'openrouter' && box.mvModels && !box.mvModels.some((m) => m.id === om)) return `OpenRouter has no model “${om}” that can use tools and see images.`;
    return {provider: p, key: k, orKey: ork, model: box.querySelector('#mv-model').value, orModel: om, keep: box.querySelector('#mv-keep').checked};
  }, (box) => { // (shows the part of the chosen provider, and fills the list of OpenRouter's models)
    const sel = box.querySelector('#mv-prov');
    const show = () => {
      box.querySelectorAll('.mv-prov').forEach((d) => { d.hidden = d.dataset.p !== sel.value; });
      box.querySelectorAll('.mv-keys').forEach((d) => { d.hidden = sel.value === ''; });
    };
    sel.addEventListener('change', () => { show(); box.querySelector(sel.value === 'openrouter' ? '#mv-orkey' : sel.value === 'anthropic' ? '#mv-key' : '#mv-prov').focus(); });
    show();
    box.querySelector('#mv-forget')?.addEventListener('click', (ev) => {
      ev.preventDefault();
      box.querySelector('#mv-key').value = ''; box.querySelector('#mv-orkey').value = '';
      ev.target.closest('p').textContent = 'The keys are forgotten when you save.';
    });
    orModels().then((list) => {
      box.mvModels = list;
      box.querySelector('#mv-ormodels').innerHTML = list.map((m) => `<option value="${esc(m.id)}">${esc(m.name)}</option>`).join('');
    }).catch(() => { box.querySelector('#mv-ornote').textContent = `The list of OpenRouter’s models could not be loaded (offline?); type the model’s id, such as ${OR_DEFAULT}.`; });
  });
  if (!v) return {ok: false};
  memKeys.anthropic = v.keep ? '' : v.key; memKeys.openrouter = v.keep ? '' : v.orKey;
  await SetChatAPIConfig(v.provider, v.model, v.orModel, v.keep ? v.key : '', v.keep ? v.orKey : '');
  cfg = await ChatAPIConfig();
  const changed = v.provider !== was;
  if (changed) { busy?.abort.abort(); messages = []; orMessages = []; }
  return {ok: true, changed};
}

// The chat methods of the panel when the chat goes to a model API (the same as the Go methods of chat.go for Claude Code).
export const apiBindings = {
  async ChatInfo() {
    await config();
    const has = !!activeKey(), or = provider() === 'openrouter';
    const name = or ? orModel() : MODELS.find(([id]) => id === model())?.[1].replace(/ \(.*/, '') || model();
    return {
      claude: true, loggedIn: has, email: has ? (or ? 'OpenRouter' : 'Claude API') : '', plan: name, method: '', api: true, busy: !!busy, signingIn: false,
      signInText: `To chat with Claude about this image, give mormiView an API key: your Claude API key, or an OpenRouter key. The chat then talks to that provider; the key stays ${cfg.desktop ? 'on this computer' : 'in this browser'}.`,
      signInLabel: 'Set API key…',
    };
  },
  async ChatLogin() {
    const r = await askProvider();
    setTimeout(() => emit(r.ok ? {kind: 'login', ok: true, changed: r.changed} : {kind: 'login', ok: false, message: 'no key was given'}), 0);
  },
  async ChatSend(text) {
    await config();
    text = String(text || '').trim();
    if (!text) throw 'there is nothing to send';
    if (!activeKey()) throw 'Set your API key first (AI → AI Provider and Key…)';
    if (busy) throw 'an answer is being written: wait for it, or stop it';
    if (provider() === 'openrouter') runOpenRouter(text);
    else run(text);
  },
  async ChatStop() { busy?.abort.abort(); },
  async ChatReset() { busy?.abort.abort(); messages = []; orMessages = []; },
};
