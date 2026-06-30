// ===== 全局状态 =====
let config = null;
let currentGameIdx = 0;
let currentModIdx = 0;

// 用数组存储 mod 数据，避免路径出现在 onclick 中
let localPlantMods = [];
let localZombieMods = [];
let installedMods = [];
let onlineMods = [];

// ===== 初始化 =====
async function init() {
  await loadConfig();
  await refreshAll();
  // 事件委托
  document.getElementById('plantUninstalled').onclick = e => {
    const btn = e.target.closest('button');
    if (!btn) return;
    const idx = parseInt(btn.dataset.idx);
    if (!isNaN(idx)) installLocalMod(idx);
  };
  document.getElementById('zombieUninstalled').onclick = e => {
    const btn = e.target.closest('button');
    if (!btn) return;
    const idx = parseInt(btn.dataset.idx);
    if (!isNaN(idx)) installLocalMod(idx);
  };
  document.getElementById('plantInstalled').onclick = e => {
    const btn = e.target.closest('button');
    if (!btn) return;
    const idx = parseInt(btn.dataset.idx);
    if (!isNaN(idx)) uninstallMod(idx);
  };
  document.getElementById('zombieInstalled').onclick = e => {
    const btn = e.target.closest('button');
    if (!btn) return;
    const idx = parseInt(btn.dataset.idx);
    if (!isNaN(idx)) uninstallMod(idx);
  };
  document.getElementById('onlineModList').onclick = e => {
    const btn = e.target.closest('button');
    if (!btn) return;
    const idx = parseInt(btn.dataset.idx);
    if (!isNaN(idx)) installOnlineMod(idx);
  };
}

// ===== API 调用 =====
async function api(url, opts = {}) {
  const res = await fetch(url, opts);
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
}

async function apiPost(url, body) {
  return api(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  });
}

// ===== 配置 =====
async function loadConfig() {
  config = await api('/api/config');
  renderConfig();
}

async function saveConfig() {
  await apiPost('/api/config', config);
}

function renderConfig() {
  const sel = $('#gamePath');
  sel.innerHTML = '';
  config.game_path.forEach((g, i) => {
    const opt = document.createElement('option');
    opt.value = i;
    opt.textContent = `${g.path} [${g.version}]`;
    sel.appendChild(opt);
  });
  if (config.game_path.length === 0) {
    sel.innerHTML = '<option value="">-- 请添加游戏目录 --</option>';
  }

  const msel = $('#modPath');
  msel.innerHTML = '';
  config.mod_path.forEach((m, i) => {
    const opt = document.createElement('option');
    opt.value = i;
    opt.textContent = m;
    msel.appendChild(opt);
  });
  if (config.mod_path.length === 0) {
    msel.innerHTML = '<option value="">-- 请添加 MOD 目录 --</option>';
  }

  $('#serverUrl').value = config.server_url || '';
}

// ===== 刷新 =====
async function refreshAll() {
  await Promise.all([
    refreshVersions(),
    refreshBepInEx(),
    refreshMods(),
    refreshModifier(),
    refreshOnlineFilters()
  ]);
}

async function refreshVersions() {
  if (!currentModPath()) return;
  try {
    const versions = await api('/api/versions');
    const sel = $('#version');
    sel.innerHTML = '';
    versions.forEach(v => {
      const opt = document.createElement('option');
      opt.value = v;
      opt.textContent = v;
      if (currentGame() && currentGame().version === v) opt.selected = true;
      sel.appendChild(opt);
    });
    if (versions.length === 0) {
      sel.innerHTML = '<option value="">-- 无可用版本 --</option>';
    }
  } catch (e) { console.error(e); }
}

async function refreshBepInEx() {
  const gp = currentGamePath();
  if (!gp) return;
  try {
    const res = await api(`/api/bepinex/check?path=${encodeURIComponent(gp)}`);
    $('#bepinexStatus').textContent = res.installed ? '✅ 已安装' : '❌ 未安装';
    $('#btnBepInEx').textContent = res.installed ? '卸载 BepInEx' : '安装 BepInEx';
  } catch (e) {
    $('#bepinexStatus').textContent = '检测失败';
  }
}

async function refreshMods() {
  const mp = currentModPath();
  const ver = currentVersion();
  const gp = currentGamePath();

  // 并行加载本地和已安装
  const [local, installed] = await Promise.all([
    (mp && ver) ? api(`/api/mods/local?modPath=${encodeURIComponent(mp)}&version=${encodeURIComponent(ver)}`).catch(() => null) : null,
    gp ? api(`/api/mods/installed?path=${encodeURIComponent(gp)}`).catch(() => null) : null
  ]);

  // 收集所有本地 MOD（用于后续交叉引用）
  localPlantMods = local ? (local['植物MOD'] || []) : [];
  localZombieMods = local ? (local['僵尸MOD'] || []) : [];
  const allLocalMods = [...localPlantMods, ...localZombieMods];

  // 更新作者筛选下拉
  const authorSet = new Set(allLocalMods.map(m => m.author).filter(Boolean));
  const authorSel = $('#authorFilter');
  const curAuthor = authorSel.value;
  authorSel.innerHTML = '<option value="">全部作者</option>';
  [...authorSet].sort().forEach(a => {
    const opt = document.createElement('option');
    opt.value = a; opt.textContent = a;
    if (a === curAuthor) opt.selected = true;
    authorSel.appendChild(opt);
  });

  // 构建 dll → 本地 MOD 映射表
  const dllToLocal = {};
  for (const m of allLocalMods) {
    for (const dll of (m.dll_names || [])) {
      dllToLocal[dll] = m;
    }
  }

  // 已安装 → 用本地 MOD 的中文名和目录名补全
  installedMods = [];
  if (installed) {
    for (const cat of ['植物MOD', '僵尸MOD', '皮肤MOD', '关卡', '其他']) {
      for (const m of (installed[cat] || [])) {
        // 从 source_path 提取实际文件名
        const parts = m.source_path ? m.source_path.replace(/\\/g, '/').split('/') : [];
        const fileName = parts[parts.length - 1];
        // 查找对应本地 MOD 的中文名
        const localMatch = dllToLocal[fileName] || dllToLocal[m.dir_name];
        if (localMatch) {
          m.display_name = localMatch.display_name;
          m.dir_name = localMatch.dir_name;
          m.author = localMatch.author;
        }
        installedMods.push(m);
      }
    }
  }

  // 构建已安装 dll 文件名集合
  const installedDlls = new Set(installedMods.map(m => {
    const parts = m.source_path ? m.source_path.replace(/\\/g, '/').split('/') : [];
    return parts[parts.length - 1];
  }));

  // 作者筛选
  const filterAuthor = $('#authorFilter').value;

  function isInstalled(mod) {
    if (!mod.dll_names || mod.dll_names.length === 0) {
      return installedDlls.has(mod.dir_name);
    }
    return mod.dll_names.some(dll => installedDlls.has(dll));
  }

  function filterMods(mods) {
    return mods.filter(m => {
      if (filterAuthor && m.author !== filterAuthor) return false;
      return true;
    });
  }

  renderLocalModList('plantUninstalled', 'plantUninstalledCount',
    filterMods(localPlantMods.filter(m => !isInstalled(m))));
  renderLocalModList('zombieUninstalled', 'zombieUninstalledCount',
    filterMods(localZombieMods.filter(m => !isInstalled(m))));

  // 已安装列表
  renderInstalledModList('plantInstalled', 'plantInstalledCount',
    filterMods(installedMods.filter(m => m.category === '植物MOD')));
  renderInstalledModList('zombieInstalled', 'zombieInstalledCount',
    filterMods(installedMods.filter(m => m.category === '僵尸MOD')));
}

async function refreshModifier() {
  const mp = currentModPath();
  const ver = currentVersion();
  if (!mp || !ver) return;
  try {
    const pack = await api(`/api/modifier/find?modPath=${encodeURIComponent(mp)}&version=${encodeURIComponent(ver)}`);
    $('#modifierStatus').textContent = `✅ 找到: ${pack.file_name} (${pack.author})`;
    $('#modifierStatus').dataset.pack = JSON.stringify(pack);
  } catch (e) {
    $('#modifierStatus').textContent = `⚠ 未找到匹配版本 ${ver} 的修改器`;
    delete $('#modifierStatus').dataset.pack;
  }
}

async function refreshOnlineFilters() {
  if (!config.server_url) return;
  try {
    const versions = await api('/api/online/versions');
    fillSelect($('#onlineVer'), versions, true);
    const authors = await api('/api/online/authors');
    fillSelect($('#onlineAuthor'), authors, true);
    await fetchOnlineMods();
  } catch (e) { console.error(e); }
}

// ===== 列表渲染 (使用 data-idx 替代 onclick) =====
function renderLocalModList(listId, countId, items) {
  const ul = document.getElementById(listId);
  const count = document.getElementById(countId);
  ul.innerHTML = '';
  count.textContent = items.length;
  items.forEach((item, idx) => {
    const li = document.createElement('li');
    li.innerHTML = `<span class="mod-name" title="${escHtml(item.dir_name)}">${escHtml(item.display_name)}</span>
      <span class="mod-author">${escHtml(item.author)}</span>
      <button data-idx="${idx}">安装</button>`;
    ul.appendChild(li);
  });
}

function renderInstalledModList(listId, countId, items) {
  const ul = document.getElementById(listId);
  const count = document.getElementById(countId);
  ul.innerHTML = '';
  count.textContent = items.length;
  items.forEach((item, idx) => {
    const li = document.createElement('li');
    li.innerHTML = `<span class="mod-name">${escHtml(item.display_name)}</span>
      <button class="danger" data-idx="${idx}">移除</button>`;
    ul.appendChild(li);
  });
}

async function fetchOnlineMods() {
  const ver = $('#onlineVer').value;
  const author = $('#onlineAuthor').value;
  const type = $('#onlineType').value;
  const ul = $('#onlineModList');

  if (!config.server_url) {
    ul.innerHTML = '<li>未配置服务器地址</li>';
    return;
  }

  try {
    onlineMods = await api(`/api/online/mods?ver=${encodeURIComponent(ver)}&author=${encodeURIComponent(author)}&type=${encodeURIComponent(type)}`);
    ul.innerHTML = '';
    onlineMods.forEach((m, idx) => {
      const li = document.createElement('li');
      li.innerHTML = `<span class="mod-name" title="${escHtml(m.name_en)}">${escHtml(m.name_cn)}</span>
        <span class="mod-author">${escHtml(m.author)}</span>
        <span class="mod-type">${escHtml(m.mod_type)}</span>
        <button data-idx="${idx}">下载安装</button>`;
      ul.appendChild(li);
    });
  } catch (e) {
    ul.innerHTML = `<li class="error">加载失败: ${e.message}</li>`;
  }
}

// ===== 操作 =====
async function toggleBepInEx() {
  const gp = currentGamePath();
  if (!gp) return alert('请先选择游戏目录');
  const btn = $('#btnBepInEx');
  if (btn.textContent.includes('卸载')) {
    if (!confirm('确定卸载 BepInEx？将删除框架文件，保留游戏本体。')) return;
    await api(`/api/bepinex/uninstall?path=${encodeURIComponent(gp)}`);
  } else {
    await api(`/api/bepinex/install?path=${encodeURIComponent(gp)}`);
  }
  await refreshBepInEx();
  await refreshMods();
}

// idx 参数来自 data-idx，而非内联路径字符串
async function installLocalMod(idx) {
  const gp = currentGamePath();
  if (!gp) return alert('请先选择游戏目录');

  const item = localPlantMods[idx] !== undefined
    ? localPlantMods[idx]
    : localZombieMods[idx];
  if (!item) return alert('MOD 数据不存在');

  try {
    await apiPost('/api/mods/install', { game_path: gp, item: item });
    await refreshMods();
  } catch (e) {
    alert('安装失败: ' + e.message);
  }
}

async function uninstallMod(idx) {
  const item = installedMods[idx];
  if (!item) return alert('MOD 数据不存在');
  if (!confirm(`确定移除 "${item.display_name}"？`)) return;
  try {
    await apiPost('/api/mods/uninstall', item);
    await refreshMods();
  } catch (e) {
    alert('移除失败: ' + e.message);
  }
}

async function installModifier() {
  const gp = currentGamePath();
  if (!gp) return alert('请先选择游戏目录');
  const packData = $('#modifierStatus').dataset.pack;
  if (!packData) return alert('未找到可用的修改器');
  try {
    const pack = JSON.parse(packData);
    await apiPost('/api/modifier/install', { game_path: gp, pack: pack });
    alert('修改器安装完成');
  } catch (e) {
    alert('安装失败: ' + e.message);
  }
}

async function installOnlineMod(idx) {
  const info = onlineMods[idx];
  if (!info) return alert('MOD 数据不存在');
  const gp = currentGamePath();
  if (!gp) return alert('请先选择游戏目录');
  try {
    await apiPost('/api/online/install', { game_path: gp, info: info });
    await refreshMods();
    alert('安装完成');
  } catch (e) {
    alert('安装失败: ' + e.message);
  }
}

// ===== 路径管理 =====
async function selectFolder() {
  try {
    const res = await api('/api/select-folder');
    return res.path;
  } catch (e) {
    console.error('选择目录失败:', e);
    return '';
  }
}

async function addGamePath() {
  const path = await selectFolder();
  if (!path) return;
  const version = prompt('请输入对应的游戏版本（如 3.7）：');
  if (!version) return;
  config.game_path.push({ path, version });
  await saveConfig();
  renderConfig();
  await refreshAll();
}

async function addModPath() {
  const path = await selectFolder();
  if (!path) return;
  config.mod_path.push(path);
  await saveConfig();
  renderConfig();
  await refreshAll();
}

async function onGamePathChange() {
  currentGameIdx = parseInt($('#gamePath').value) || 0;
  await refreshAll();
}

async function onVersionChange() {
  await refreshMods();
  await refreshModifier();
}

async function onModPathChange() {
  currentModIdx = parseInt($('#modPath').value) || 0;
  await refreshAll();
}

async function onServerUrlChange() {
  config.server_url = $('#serverUrl').value;
  await saveConfig();
  await refreshOnlineFilters();
}

// ===== 打开目录 =====
async function openDir(path) {
  if (!path) return;
  try {
    await api(`/api/open-dir?path=${encodeURIComponent(path)}`);
  } catch (e) {
    console.error('打开目录失败:', e);
  }
}

// ===== 辅助 =====
function $(sel) { return document.querySelector(sel); }
function escHtml(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; }

function currentGame() { return config.game_path[currentGameIdx] || null; }
function currentGamePath() { const g = currentGame(); return g ? g.path : ''; }
function currentVersion() { return $('#version').value || (currentGame() ? currentGame().version : ''); }
function currentModPath() { return config.mod_path[currentModIdx] || ''; }

function fillSelect(sel, items, keepFirst) {
  const val = keepFirst ? sel.value : '';
  sel.innerHTML = keepFirst ? sel.options[0]?.outerHTML || '' : '';
  items.forEach(item => {
    const opt = document.createElement('option');
    opt.value = item;
    opt.textContent = item;
    if (item === val) opt.selected = true;
    sel.appendChild(opt);
  });
}

init();
