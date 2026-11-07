'use strict';

let currentConfig = null;
let selectedLocale = 'en-GB';
let translations = {};
let autoFilled = false;
let citySearchGeneration = 0;
const t = key => translations[key] || key;
const format = (key, ...args) => t(key).replace(/%[dsv]/g, () => String(args.shift()));
const el = id => document.getElementById(id);
const displayFields = {
  'df-city': 'showCity', 'df-icon': 'showIcon', 'df-temp': 'showTemp',
  'df-desc': 'showDesc', 'df-humidity': 'showHumidity', 'df-wind': 'showWind',
  'df-time': 'showTime', 'df-date': 'showDate', 'df-gust': 'showWindGust',
  'df-dew': 'showDewPoint', 'df-pressure': 'showPressure', 'df-uv': 'showUVIndex'
};
const pollutionFields = {
  'pf-aqi': 'showAQI', 'pf-co': 'showCO', 'pf-no': 'showNO', 'pf-no2': 'showNO2',
  'pf-o3': 'showO3', 'pf-so2': 'showSO2', 'pf-nh3': 'showNH3',
  'pf-pm25': 'showPM25', 'pf-pm10': 'showPM10', 'pf-ticker': 'showTicker'
};
const tickerFields = {
  'tf-aqi': 'tickerAQI', 'tf-co': 'tickerCO', 'tf-no': 'tickerNO', 'tf-no2': 'tickerNO2',
  'tf-o3': 'tickerO3', 'tf-so2': 'tickerSO2', 'tf-nh3': 'tickerNH3',
  'tf-pm25': 'tickerPM25', 'tf-pm10': 'tickerPM10'
};
const translationTargets = {
  '[data-tab="tab-cities"] .nav-label': 'settings.tab.locations',
  '[data-tab="tab-appearance"] .nav-label': 'settings.tab.appearance',
  '[data-tab="tab-display"] .nav-label': 'settings.tab.widget',
  '[data-tab="tab-ticker"] .nav-label': 'settings.tab.ticker',
  '[data-tab="tab-api"] .nav-label': 'settings.tab.provider',
  '[data-tab="tab-language"] .nav-label': 'settings.tab.language',
  '[data-tab="tab-about"] .nav-label': 'settings.tab.about',
  '.app-subtitle': 'settings.title',
  '#tab-cities .pane-desc': 'settings.locations.savedSubtitle',
  '#demo-banner-title': 'settings.locations.demoBanner.title',
  '#demo-banner-subtitle': 'settings.locations.demoBanner.subtitle',
  '#demo-banner-pro': 'settings.locations.demoBanner.proBtn',
  '#demo-banner-free': 'settings.locations.demoBanner.freeBtn',
  '#btn-save': 'settings.save', '#btn-cancel': 'settings.cancel',
  '#tab-cities .pane-title': 'settings.locations.savedTitle',
  '#btn-add-city': 'settings.locations.addBtn', '#btn-search-city': 'settings.locations.searchBtn',
  'label[for="new-city-name"] span': 'settings.locations.nameLabel',
  'label[for="new-city-region"] span': 'settings.locations.regionLabel',
  'label[for="new-city-tz"] span': 'settings.locations.tzLabel',
  'label[for="new-city-lat"] span': 'settings.locations.latLabel',
  'label[for="new-city-lon"] span': 'settings.locations.lonLabel',
  '#tab-display .pane-title': 'settings.display.title',
  '#tab-display .pane-desc': 'settings.display.subtitle',
  '#tab-display .card-subtitle:first-child': 'settings.display.title',
  '#df-city': 'settings.display.city', '#df-icon': 'settings.display.icon',
  '#df-temp': 'settings.display.temp', '#df-desc': 'settings.display.desc',
  '#df-time': 'settings.display.time', '#df-date': 'settings.display.date',
  '#df-humidity': 'settings.display.humidity', '#df-wind': 'settings.display.wind',
  '#df-gust': 'settings.display.windGust', '#df-dew': 'settings.display.dewPoint',
  '#df-pressure': 'settings.display.pressure', '#df-uv': 'settings.display.uvIndex',
  '#pf-aqi': 'settings.pollution.aqi', '#pf-co': 'settings.pollution.co',
  '#pf-no': 'settings.pollution.no', '#pf-no2': 'settings.pollution.no2',
  '#pf-o3': 'settings.pollution.o3', '#pf-so2': 'settings.pollution.so2',
  '#pf-nh3': 'settings.pollution.nh3', '#pf-pm25': 'settings.pollution.pm2_5',
  '#pf-pm10': 'settings.pollution.pm10', '#pf-ticker': 'settings.pollution.ticker',
  '#tf-aqi': 'settings.pollution.aqi', '#tf-co': 'settings.pollution.co',
  '#tf-no': 'settings.pollution.no', '#tf-no2': 'settings.pollution.no2',
  '#tf-o3': 'settings.pollution.o3', '#tf-so2': 'settings.pollution.so2',
  '#tf-nh3': 'settings.pollution.nh3', '#tf-pm25': 'settings.pollution.pm2_5',
  '#tf-pm10': 'settings.pollution.pm10',
  '#tab-ticker .pane-title': 'settings.ticker.title',
  '#tab-ticker .pane-desc': 'settings.ticker.subtitle',
  '#ticker-section-title': 'settings.ticker.title',
  '#ticker-locked-message': 'settings.pollution.proRequired',
  '#ticker-get-pro': 'settings.locations.demoBanner.proBtn',
  '#ticker-profile-hint': 'settings.pollution.profile.subtitle',
  '#ticker-pollutants-title': 'settings.ticker.pollutantsTitle',
  '#tab-ticker .card > .setting-hint': 'settings.pollution.proNote',
  'label[for="sel-ticker-profile"]': 'settings.pollution.profile.title',
  '#sel-ticker-profile option[value="everyday"]': 'settings.pollution.profile.everyday',
  '#sel-ticker-profile option[value="active"]': 'settings.pollution.profile.active',
  '#sel-ticker-profile option[value="family"]': 'settings.pollution.profile.family',
  '#sel-ticker-profile option[value="sensitive"]': 'settings.pollution.profile.sensitive',
  '#sel-ticker-profile option[value="ventilation"]': 'settings.pollution.profile.ventilation',
  'label[for="sel-icon-theme"]': 'settings.icons.title',
  '#tab-language .pane-title': 'settings.language.title',
  '#tab-language .pane-desc': 'settings.language.subtitle',
  'label[for="sel-monitor"]': 'settings.monitor.label',
  'label[for="sel-widget-scale"]': 'settings.widgetScale.title',
  '#widget-scale-hint': 'settings.widgetScale.subtitle',
  '#sel-widget-scale option[value="0"]': 'settings.widgetScale.auto',
  'label[for="font-city"]': 'settings.fontSize.cityTime',
  'label[for="font-temp"]': 'settings.fontSize.tempIcon',
  'label[for="font-conditions"]': 'settings.fontSize.conditions',
  '#sel-view-mode option[value="enhanced"]': 'settings.viewMode.enhanced',
  '#sel-view-mode option[value="simple"]': 'settings.viewMode.simple',
  '#btn-test-connection': 'settings.provider.checkConnection',
  '#tab-appearance .pane-title': 'settings.tab.appearance',
  '#tab-appearance .pane-desc': 'settings.fontSize.subtitle',
  '#tab-appearance .setting-row:nth-child(1) .setting-label': 'settings.viewMode.title',
  '#tab-appearance .setting-row:nth-child(1) .setting-hint': 'settings.viewMode.subtitle',
  '#tab-appearance .setting-row:nth-child(2) .setting-label': 'settings.transparency.title',
  '#tab-appearance .setting-row:nth-child(3) .setting-label': 'settings.noBackground.checkbox',
  '#tab-appearance .setting-row:nth-child(3) .setting-hint': 'settings.transparency.subtitle',
  '#sel-icon-theme option[value="new"]': 'settings.icons.new',
  '#sel-icon-theme option[value="original"]': 'settings.icons.original',
  '#tab-appearance .setting-row:nth-last-child(3) .setting-label': 'settings.temperature.title',
  '#tab-appearance .setting-row:nth-last-child(3) .setting-hint': 'settings.temperature.subtitle',
  '#sel-temp-unit option[value="celsius"]': 'settings.temperature.celsius',
  '#sel-temp-unit option[value="fahrenheit"]': 'settings.temperature.fahrenheit',
  '#sel-temp-unit option[value="kelvin"]': 'settings.temperature.kelvin',
  '#tab-appearance .setting-row:nth-last-child(2) .setting-label': 'settings.windspeed.title',
  '#tab-appearance .setting-row:nth-last-child(2) .setting-hint': 'settings.windspeed.subtitle',
  '#tab-appearance .setting-row:last-child .setting-label': 'settings.position.title',
  '#tab-appearance .setting-row:last-child .setting-hint': 'settings.position.subtitle',
  '#btn-pos-apply': 'settings.windows.apply',
  '#sel-corner option[value="top-left"]': 'settings.position.topLeft',
  '#sel-corner option[value="top-right"]': 'settings.position.topRight',
  '#sel-corner option[value="bottom-left"]': 'settings.position.bottomLeft',
  '#sel-corner option[value="bottom-right"]': 'settings.position.bottomRight',
  '#pollution-title': 'settings.pollution.title',
  '#pollution-locked-message': 'settings.pollution.proRequired',
  '#pollution-get-pro': 'settings.locations.demoBanner.proBtn',
  '#tab-display .card > .setting-hint': 'settings.pollution.proNote',
  '#tab-api .pane-title': 'settings.provider.title',
  '#tab-api .pane-desc': 'settings.provider.subtitle',
  '#remote-settings .setting-row:first-child .setting-label': 'settings.provider.label',
  '#remote-settings .setting-row:first-child .setting-hint': 'settings.provider.subtitle',
  '#row-api-key .setting-label': 'settings.provider.apiKeyLabel',
  '#row-api-key .setting-hint': 'settings.provider.note',
  '#activation-title': 'settings.provider.apiKeyActivation.title',
  '#activation-message': 'settings.provider.apiKeyActivation.message',
  '#link-get-free-api': 'settings.provider.getFreeApi',
  '#link-get-pro-api': 'settings.provider.getProApi',
  '#row-refresh-interval .setting-label': 'settings.interval.title',
  '#row-refresh-interval .setting-hint': 'settings.interval.subtitle',
  '#tab-about .pane-title': 'settings.tab.about',
  '#tab-about .pane-desc': 'settings.about.description',
  '#tab-about .setting-label': 'settings.startup.autostart',
  '#tab-about .setting-hint': 'settings.startup.subtitle',
  '#tab-about .about-links a:first-child': 'settings.about.websiteLabel',
  '#tab-about .about-links a:nth-of-type(2)': 'settings.windows.support'
};

async function applyLanguage(locale) {
  const keys = [...Object.values(translationTargets), ...Object.values(placeholderTargets),
    'settings.locations.latLabel', 'settings.locations.lonLabel', 'settings.locations.tzLabel',
    'settings.locations.removeBtn', 'settings.monitor.format', 'settings.transparency.title',
    'settings.dialog.savedMsg', 'settings.provider.checkingConnection', 'settings.provider.connectionSuccessTitle',
    'error.settings.cityNameRequiredSearch', 'error.settings.regionRequiredEww', 'error.settings.apiKeyRequired',
    'error.settings.searchFailed', 'error.settings.saveFailed', 'error.settings.connectionFailed',
    'error.settings.autoStartFailed', 'settings.windows.autofillRequired', 'settings.windows.error', 'settings.windows.apply'];
  const values = await window.goGetTranslations(locale, keys);
  translations = values;
  document.documentElement.lang = locale;
  document.title = t('settings.title');
  Object.entries(translationTargets).forEach(([selector, key]) => {
    const target = document.querySelector(selector);
    if (!target || !values[key] || values[key] === key) return;
    if (target.type === 'checkbox') {
      target.parentElement.lastChild.textContent = ' ' + values[key];
    } else {
      target.textContent = values[key];
    }
  });
  Object.entries(placeholderTargets).forEach(([selector, key]) => {
    document.querySelector(selector).placeholder = t(key);
  });
  updateOpacityHint();
  renderCityList();
  updateLanguageSelection();
  Array.from(el('sel-monitor').options).forEach((option, i) => { option.textContent = format('settings.monitor.format', i + 1); });
}

const placeholderTargets = {
  '#new-city-name': 'settings.locations.namePlaceholder',
  '#new-city-region': 'settings.locations.regionPlaceholder',
  '#new-city-tz': 'settings.locations.tzLabel',
  '#new-city-lat': 'settings.locations.latLabel',
  '#new-city-lon': 'settings.locations.lonLabel',
  '#input-api-key': 'settings.provider.apiKeyPlaceholder'
};
function selectTab(item) {
  document.querySelectorAll('.nav-item').forEach(i => i.classList.remove('active'));
  document.querySelectorAll('.tab-pane').forEach(p => p.classList.remove('active'));
  item.classList.add('active');
  el(item.dataset.tab).classList.add('active');
}
document.querySelectorAll('.nav-item').forEach(item => item.addEventListener('click', () => selectTab(item)));

function updateDemoBanner() {
  el('demo-banner').hidden = !!(currentConfig?.apiConfig?.apiKey || currentConfig?.databaseConfig?.host);
}

function updatePollutionControls() {
  const isPro = !!(currentConfig?.apiConfig?.provider === 'easyweatherwidget' && currentConfig.apiConfig.apiKey);
  el('pollution-controls').disabled = !isPro;
  el('pollution-unlock').hidden = isPro;
  if (isPro) el('pollution-controls').removeAttribute('aria-describedby');
  else el('pollution-controls').setAttribute('aria-describedby', 'pollution-locked-message');

  el('ticker-controls').disabled = !isPro;
  el('ticker-unlock').hidden = isPro;
  if (isPro) el('ticker-controls').removeAttribute('aria-describedby');
  else el('ticker-controls').setAttribute('aria-describedby', 'ticker-locked-message');
}

el('demo-banner-pro').addEventListener('click', () => {
  selectTab(document.querySelector('[data-tab="tab-api"]'));
  el('link-get-pro-api').click();
});
el('pollution-get-pro').addEventListener('click', () => el('demo-banner-pro').click());
el('ticker-get-pro').addEventListener('click', () => el('demo-banner-pro').click());
el('demo-banner-free').addEventListener('click', () => {
  selectTab(document.querySelector('[data-tab="tab-api"]'));
  el('sel-provider').value = 'openweathermap';
  el('sel-provider').dispatchEvent(new Event('change'));
  el('input-api-key').focus();
});

function showToast(message, isError = false) {
  const toast = el('toast');
  toast.textContent = message;
  toast.style.borderColor = isError ? '#ff6b6b' : 'var(--accent)';
  toast.classList.remove('hidden');
  if (!isError) setTimeout(() => toast.classList.add('hidden'), 3000);
}

function populateUI(cfg) {
  currentConfig = cfg;
  currentConfig.cities ||= [];
  renderCityList();
  el('sel-view-mode').value = cfg.viewMode || 'enhanced';
  el('slider-opacity').value = cfg.opacity || 100;
  updateOpacityHint();
  el('chk-no-bg').checked = !!cfg.noBackground;
  el('sel-temp-unit').value = cfg.temperatureUnit || 'celsius';
  el('sel-wind-unit').value = cfg.windSpeedUnit || 'kmh';
  el('sel-corner').value = cfg.cornerPosition || 'top-right';
  el('sel-icon-theme').value = cfg.iconTheme || 'new';
  selectedLocale = cfg.locale || 'en-GB';
  el('sel-monitor').value = String(cfg.monitorIndex || 0);
  el('sel-widget-scale').value = String(cfg.widgetScale || 0);
  el('font-city').value = cfg.fontSizeCityTime || 14;
  el('font-temp').value = cfg.fontSizeTempIcon || 32;
  el('font-conditions').value = cfg.fontSizeConditions || 10;
  Object.entries(displayFields).forEach(([id, key]) => {
    el(id).checked = cfg.displayFields ? !!cfg.displayFields[key] : true;
  });
  Object.entries(pollutionFields).forEach(([id, key]) => {
    el(id).checked = !!cfg.pollutionFields?.[key];
  });
  Object.entries(tickerFields).forEach(([id, key]) => {
    el(id).checked = !!cfg.pollutionFields?.[key];
  });
  el('sel-ticker-profile').value = cfg.pollutionFields?.tickerProfile || 'everyday';
  el('sel-provider').value = cfg.apiConfig?.provider || 'easyweatherwidget';
  el('input-api-key').value = cfg.apiConfig?.apiKey || '';
  el('sel-refresh-interval').value = cfg.refreshInterval;
  updateActivationCard();
  updateDemoBanner();
  updatePollutionControls();
}

function renderCityList() {
  const list = el('city-list');
  list.replaceChildren();
  currentConfig.cities.forEach((city, index) => {
    const item = document.createElement('li');
    item.className = 'city-item';
    const text = document.createElement('div');
    const name = document.createElement('div');
    name.className = 'city-item-name';
    name.textContent = [city.name, city.region].filter(Boolean).join(', ');
    const subtitle = document.createElement('div');
    subtitle.className = 'city-item-sub';
    subtitle.textContent = `${city.timezone || 'UTC'} • ${city.latitude || 0}, ${city.longitude || 0}`;
    text.append(name, subtitle);
    const button = document.createElement('button');
    button.className = 'btn-remove-city';
    button.textContent = t('settings.locations.removeBtn');
    button.addEventListener('click', () => {
      currentConfig.cities.splice(index, 1);
      renderCityList();
    });
    item.append(text, button);
    list.appendChild(item);
  });
}

el('btn-add-city').addEventListener('click', () => {
  const name = el('new-city-name').value.trim();
  const region = el('new-city-region').value.trim();
  const timezone = el('new-city-tz').value.trim();
  const lat = el('new-city-lat').value;
  const lon = el('new-city-lon').value;
  if (!autoFilled || !name || !region || !timezone || lat === '' || lon === '') {
    showToast(t('settings.windows.autofillRequired'), true);
    return;
  }
  const latitude = Number(lat), longitude = Number(lon);
  if (!Number.isFinite(latitude) || !Number.isFinite(longitude) || Math.abs(latitude) > 90 || Math.abs(longitude) > 180) {
    showToast(format('error.settings.invalidLat', lat) + ' / ' + format('error.settings.invalidLon', lon), true);
    return;
  }
  currentConfig.cities.push({name, region, timezone, latitude, longitude});
  ['new-city-name', 'new-city-region', 'new-city-tz', 'new-city-lat', 'new-city-lon'].forEach(id => { el(id).value = ''; });
  autoFilled = false;
  renderCityList();
});

function clearCityAutofill() {
  citySearchGeneration++;
  autoFilled = false;
  ['new-city-tz', 'new-city-lat', 'new-city-lon'].forEach(id => { el(id).value = ''; });
}
['new-city-name', 'new-city-region'].forEach(id => el(id).addEventListener('input', clearCityAutofill));
['sel-provider', 'input-api-key'].forEach(id => el(id).addEventListener('change', clearCityAutofill));

el('btn-search-city').addEventListener('click', async () => {
  clearCityAutofill();
  const name = el('new-city-name').value.trim(), region = el('new-city-region').value.trim();
  if (!name) { showToast(t('error.settings.cityNameRequiredSearch'), true); return; }
  if (!region) { showToast(t('settings.windows.autofillRequired'), true); return; }
  if (!el('input-api-key').value.trim()) { showToast(t('error.settings.apiKeyRequired'), true); return; }
  const generation = citySearchGeneration;
  const button = el('btn-search-city');
  button.disabled = true;
  try {
    const city = await window.goSearchCity(name, region,
      el('sel-provider').value, el('input-api-key').value.trim(), selectedLocale);
    if (generation !== citySearchGeneration) return;
    el('new-city-name').value = city.name;
    el('new-city-region').value = city.region;
    el('new-city-tz').value = city.timezone;
    el('new-city-lat').value = city.latitude;
    el('new-city-lon').value = city.longitude;
    autoFilled = true;
  } catch (err) {
    if (generation === citySearchGeneration) showToast(format('error.settings.searchFailed', err), true);
  } finally {
    button.disabled = false;
  }
});

function updateOpacityHint() { el('opacity-hint').textContent = `${t('settings.transparency.title')} (${el('slider-opacity').value}%)`; }
el('slider-opacity').addEventListener('input', updateOpacityHint);

let apiKeyVisible = false;
function maskApiKey(key) {
  if (!key) return '';
  return key.replace(/[^-]/g, '•');
}
function updateActivationCard() {
  const key = el('input-api-key').value.trim();
  const isProProvider = el('sel-provider').value === 'easyweatherwidget';
  const showCard = key.length > 0 || isProProvider;
  el('activation-card').hidden = !showCard;
  const keyEl = el('activation-key');
  if (key.length > 0) {
    keyEl.hidden = false;
    keyEl.textContent = apiKeyVisible ? key : maskApiKey(key);
  } else {
    keyEl.hidden = true;
    keyEl.textContent = '';
  }
}
el('btn-toggle-api-key').addEventListener('click', () => {
  apiKeyVisible = !apiKeyVisible;
  el('input-api-key').type = apiKeyVisible ? 'text' : 'password';
  el('btn-toggle-api-key').setAttribute('aria-pressed', String(apiKeyVisible));
  el('icon-key-show').hidden = apiKeyVisible;
  el('icon-key-hide').hidden = !apiKeyVisible;
  updateActivationCard();
});
el('input-api-key').addEventListener('input', updateActivationCard);
el('sel-provider').addEventListener('change', updateActivationCard);

el('link-get-pro-api').addEventListener('click', async e => {
  e.preventDefault();
  try {
    const clientRef = await window.goGetProAPI(el('input-api-key').value.trim());
    el('sel-provider').value = 'easyweatherwidget';
    el('input-api-key').value = clientRef;
    if (currentConfig) {
      currentConfig.apiConfig = {...currentConfig.apiConfig, provider: 'easyweatherwidget', apiKey: clientRef};
      currentConfig.dataSource = 'remote_api';
      delete currentConfig.databaseConfig;
    }
    updateActivationCard();
    updateDemoBanner();
    updatePollutionControls();
  } catch (err) {
    showToast(format('settings.windows.error', err), true);
  }
});

el('link-get-free-api').addEventListener('click', async e => {
  e.preventDefault();
  try {
    await window.goOpenURL('https://openweathermap.org/');
  } catch (err) {
    showToast(format('settings.windows.error', err), true);
  }
});

document.querySelectorAll('#tab-about .about-links a').forEach(link => {
  link.addEventListener('click', async e => {
    e.preventDefault();
    try { await window.goOpenURL(link.href); } catch (_) {}
  });
});

function updateLanguageSelection() {
  el('language-list').querySelectorAll('button').forEach(button => button.setAttribute('aria-checked', String(button.dataset.locale === selectedLocale)));
}
function renderLanguages(locales) {
  const list = el('language-list');
  const order = ['en-GB', 'es-ES', 'fr-FR', 'de-DE', 'it-IT', 'pt-BR', 'nl-NL', 'pl-PL', 'tr-TR', 'ta-IN', 'ja-JP', 'zh-CN'];
  locales.sort((a, b) => {
    const ai = order.indexOf(a.Code), bi = order.indexOf(b.Code);
    return (ai < 0 ? order.length : ai) - (bi < 0 ? order.length : bi);
  }).forEach(locale => {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'language-choice';
    button.dataset.locale = locale.Code;
    button.setAttribute('role', 'radio');
    const flag = document.createElement('img');
    flag.src = locale.Flag;
    flag.alt = '';
    const text = document.createElement('span');
    const nativeName = document.createElement('strong');
    const englishName = document.createElement('small');
    nativeName.textContent = locale.DisplayName;
    englishName.textContent = new Intl.DisplayNames(['en'], {type: 'language'}).of(locale.Code.split('-')[0]);
    text.append(nativeName, englishName);
    button.append(flag, text);
    button.addEventListener('click', async () => {
      list.inert = true;
      el('btn-save').disabled = true;
      el('btn-cancel').disabled = true;
      try {
        await applyLanguage(locale.Code);
        selectedLocale = locale.Code;
        updateLanguageSelection();
        el('toast').classList.add('hidden');
      } catch (err) { showToast(format('settings.windows.error', err), true); }
      finally {
        list.inert = false;
        el('btn-save').disabled = false;
        el('btn-cancel').disabled = false;
      }
    });
    list.append(button);
  });
  updateLanguageSelection();
}

let customPos = null;

async function moveWidgetTo(x, y) {
  if (!Number.isFinite(x) || !Number.isFinite(y)) return;
  x = Math.round(x); y = Math.round(y);
  try {
    await window.goMoveWidget(x, y);
    customPos = {x, y};
    el('pos-x').value = x;
    el('pos-y').value = y;
  } catch (err) { showToast(format('settings.windows.error', err), true); }
}

function currentPos() {
  return [parseInt(el('pos-x').value, 10), parseInt(el('pos-y').value, 10)];
}
el('btn-pos-apply').addEventListener('click', () => moveWidgetTo(...currentPos()));
['pos-x', 'pos-y'].forEach(id => el(id).addEventListener('keydown', e => {
  if (e.key === 'Enter') moveWidgetTo(...currentPos());
}));
document.querySelectorAll('.btn-nudge').forEach(button => button.addEventListener('click', () => {
  const [x, y] = currentPos();
  moveWidgetTo(x + Number(button.dataset.dx), y + Number(button.dataset.dy));
}));
['sel-corner', 'sel-monitor'].forEach(id => el(id).addEventListener('change', () => { customPos = null; }));

function collectConfigFromUI() {
  const cfg = structuredClone(currentConfig);
  cfg.viewMode = el('sel-view-mode').value;
  cfg.opacity = Number(el('slider-opacity').value);
  cfg.noBackground = el('chk-no-bg').checked;
  cfg.temperatureUnit = el('sel-temp-unit').value;
  cfg.windSpeedUnit = el('sel-wind-unit').value;
  cfg.cornerPosition = el('sel-corner').value;
  cfg.monitorIndex = Number(el('sel-monitor').value);
  if (cfg.cornerPosition !== currentConfig.cornerPosition || cfg.monitorIndex !== currentConfig.monitorIndex) {
    delete cfg.customX;
    delete cfg.customY;
  }
  if (customPos) {
    cfg.customX = customPos.x;
    cfg.customY = customPos.y;
  }

  cfg.iconTheme = el('sel-icon-theme').value;
  cfg.locale = selectedLocale;
  cfg.widgetScale = Number(el('sel-widget-scale').value);
  cfg.fontSizeCityTime = Number(el('font-city').value);
  cfg.fontSizeTempIcon = Number(el('font-temp').value);
  cfg.fontSizeConditions = Number(el('font-conditions').value);
  cfg.refreshInterval = Number(el('sel-refresh-interval').value);
  cfg.dataSource = 'remote_api';
  delete cfg.databaseConfig;
  cfg.apiConfig = {...cfg.apiConfig, provider: el('sel-provider').value, apiKey: el('input-api-key').value.trim()};
  cfg.displayFields = {...cfg.displayFields};
  Object.entries(displayFields).forEach(([id, key]) => { cfg.displayFields[key] = el(id).checked; });
  cfg.pollutionFields = {...cfg.pollutionFields, tickerProfile: el('sel-ticker-profile').value};
  Object.entries(pollutionFields).forEach(([id, key]) => { cfg.pollutionFields[key] = el(id).checked; });
  Object.entries(tickerFields).forEach(([id, key]) => { cfg.pollutionFields[key] = el(id).checked; });
  return cfg;
}

let previewTimer;
let previewInFlight = Promise.resolve();
document.querySelectorAll('#tab-appearance input, #tab-appearance select, #tab-display input, #tab-display select, #tab-ticker input, #tab-ticker select').forEach(input => {
  input.addEventListener('change', () => {
    if (!currentConfig) return;
    clearTimeout(previewTimer);
    previewTimer = setTimeout(() => {
      const config = JSON.stringify(collectConfigFromUI());
      previewInFlight = previewInFlight.then(() => window.goPreviewConfig(config))
        .catch(err => showToast(format('settings.windows.error', err), true));
    }, 150);
  });
});

el('btn-save').addEventListener('click', async () => {
  if (!currentConfig) return;
  clearTimeout(previewTimer);
  const button = el('btn-save');
  button.disabled = true;
  el('btn-cancel').disabled = true;
  document.querySelector('.content-area').inert = true;
  try {
    await previewInFlight;
    const cfg = collectConfigFromUI();
    await window.goSaveConfig(JSON.stringify(cfg));
    currentConfig = cfg;
    updateDemoBanner();
    updatePollutionControls();
    showToast(t('settings.dialog.savedMsg'));
  } catch (err) {
    showToast(format('error.settings.saveFailed', err), true);
  } finally {
    button.disabled = false;
    el('btn-cancel').disabled = false;
    document.querySelector('.content-area').inert = false;
  }
});
el('btn-cancel').addEventListener('click', async () => {
  clearTimeout(previewTimer);
  el('btn-cancel').disabled = true;
  await previewInFlight;
  try { await window.goCloseSettings(); }
  catch (err) {
    el('btn-cancel').disabled = false;
    showToast(format('settings.windows.error', err), true);
  }
});
el('btn-test-connection').addEventListener('click', async () => {
  const result = el('test-result');
  result.textContent = t('settings.provider.checkingConnection');
  try {
    await window.goTestConnection(JSON.stringify(collectConfigFromUI()));
    result.textContent = t('settings.provider.connectionSuccessTitle');
  } catch (err) {
    result.textContent = format('error.settings.connectionFailed', err);
  }
});
el('chk-autostart').addEventListener('change', async e => {
  try {
    await window.goSetAutoStart(e.target.checked);
  } catch (err) {
    e.target.checked = !e.target.checked;
    showToast(format('error.settings.autoStartFailed', err), true);
  }
});
window.addEventListener('DOMContentLoaded', async () => {
  try {
    const [json, locales, monitors, autostart] = await Promise.all([
      window.goGetConfig(), window.goGetLocales(), window.goGetMonitors(), window.goGetAutoStart()
    ]);
    renderLanguages(locales);
    for (let i = 0; i < monitors; i++) el('sel-monitor').add(new Option(String(i + 1), String(i)));
    populateUI(JSON.parse(json));
    el('chk-autostart').checked = autostart;
    const [px, py] = await window.goGetPosition();
    el('pos-x').value = px;
    el('pos-y').value = py;
    await applyLanguage(selectedLocale);
  } catch (err) {
    showToast(format('settings.windows.error', err), true);
  }
});
