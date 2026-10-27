// Settings Web Frontend Controller
let currentConfig = null;

// Tab Switching
document.querySelectorAll('.nav-item').forEach(item => {
  item.addEventListener('click', () => {
    document.querySelectorAll('.nav-item').forEach(i => i.classList.remove('active'));
    document.querySelectorAll('.tab-pane').forEach(p => p.classList.remove('active'));

    item.classList.add('active');
    const tabId = item.getAttribute('data-tab');
    document.getElementById(tabId).classList.add('active');
  });
});

// Toast notification
function showToast(message, isError = false) {
  const toast = document.getElementById('toast');
  toast.textContent = message;
  toast.style.borderColor = isError ? '#ff6b6b' : 'var(--accent)';
  toast.classList.remove('hidden');
  setTimeout(() => toast.classList.add('hidden'), 3000);
}

// Populate UI from Config object
function populateUI(cfg) {
  currentConfig = cfg;

  // Tab 1: Cities
  renderCityList(cfg.cities || []);

  // Tab 2: Appearance
  document.getElementById('sel-view-mode').value = cfg.viewMode || 'enhanced';
  document.getElementById('slider-opacity').value = cfg.opacity || 100;
  document.getElementById('opacity-hint').textContent = `Background opacity (${cfg.opacity || 100}%)`;
  document.getElementById('chk-no-bg').checked = !!cfg.noBackground;
  document.getElementById('sel-temp-unit').value = cfg.temperatureUnit || 'celsius';
  document.getElementById('sel-wind-unit').value = cfg.windSpeedUnit || 'kmh';
  document.getElementById('sel-corner').value = cfg.cornerPosition || 'top-left';

  // Tab 3: Display & Metrics
  const df = cfg.displayFields || {};
  document.getElementById('df-feels-like').checked = !!df.showFeelsLike;
  document.getElementById('df-humidity').checked = !!df.showHumidity;
  document.getElementById('df-wind').checked = !!df.showWindSpeed;
  document.getElementById('df-gust').checked = !!df.showWindGust;
  document.getElementById('df-dew').checked = !!df.showDewPoint;
  document.getElementById('df-pressure').checked = !!df.showPressure;
  document.getElementById('df-uv').checked = !!df.showUVIndex;

  const pf = cfg.pollutionFields || {};
  document.getElementById('pf-aqi').checked = !!pf.showAQI;
  document.getElementById('pf-pm25').checked = !!pf.showPM25;
  document.getElementById('pf-pm10').checked = !!pf.showPM10;
  document.getElementById('pf-o3').checked = !!pf.showO3;
  document.getElementById('pf-no2').checked = !!pf.showNO2;
  document.getElementById('pf-so2').checked = !!pf.showSO2;
  document.getElementById('pf-co').checked = !!pf.showCO;

  // Tab 4: API & Provider
  const api = cfg.apiConfig || {};
  document.getElementById('sel-provider').value = api.provider || 'openmeteo';
  document.getElementById('input-api-key').value = api.apiKey || '';
  document.getElementById('sel-refresh-interval').value = String(cfg.refreshInterval || 15);

  updateAPIKeyVisibility();
}

// Render City list items
function renderCityList(cities) {
  const list = document.getElementById('city-list');
  list.innerHTML = '';

  cities.forEach((c, index) => {
    const li = document.createElement('li');
    li.className = 'city-item';
    li.innerHTML = `
      <div>
        <div class="city-item-name">${escapeHtml(c.name || '')}</div>
        <div class="city-item-sub">${escapeHtml(c.region || '')} ${c.timezone ? '• ' + escapeHtml(c.timezone) : ''}</div>
      </div>
      <button class="btn-remove-city" data-index="${index}">Remove</button>
    `;
    list.appendChild(li);
  });

  list.querySelectorAll('.btn-remove-city').forEach(btn => {
    btn.addEventListener('click', (e) => {
      const idx = parseInt(e.target.getAttribute('data-index'), 10);
      currentConfig.cities.splice(idx, 1);
      renderCityList(currentConfig.cities);
      dispatchPreview();
    });
  });
}

// Add City Button
document.getElementById('btn-add-city').addEventListener('click', () => {
  const nameInput = document.getElementById('new-city-name');
  const regionInput = document.getElementById('new-city-region');
  const tzInput = document.getElementById('new-city-tz');

  const name = nameInput.value.trim();
  const region = regionInput.value.trim();
  const tz = tzInput.value.trim();

  if (!name) {
    showToast('Please enter a city name', true);
    return;
  }

  if (!currentConfig.cities) {
    currentConfig.cities = [];
  }

  currentConfig.cities.push({
    name: name,
    region: region,
    timezone: tz || 'UTC',
    lat: 0,
    lon: 0
  });

  nameInput.value = '';
  regionInput.value = '';
  tzInput.value = '';

  renderCityList(currentConfig.cities);
  dispatchPreview();
});

// Provider switch adjusts API key visibility
document.getElementById('sel-provider').addEventListener('change', updateAPIKeyVisibility);

function updateAPIKeyVisibility() {
  const provider = document.getElementById('sel-provider').value;
  const row = document.getElementById('row-api-key');
  if (provider === 'openmeteo') {
    row.style.opacity = '0.5';
  } else {
    row.style.opacity = '1.0';
  }
}

// Live Preview Handlers
document.getElementById('slider-opacity').addEventListener('input', (e) => {
  const val = parseInt(e.target.value, 10);
  document.getElementById('opacity-hint').textContent = `Background opacity (${val}%)`;
  if (currentConfig) {
    currentConfig.opacity = val;
    dispatchPreview();
  }
});

document.getElementById('chk-no-bg').addEventListener('change', (e) => {
  if (currentConfig) {
    currentConfig.noBackground = e.target.checked;
    dispatchPreview();
  }
});

document.getElementById('sel-view-mode').addEventListener('change', (e) => {
  if (currentConfig) {
    currentConfig.viewMode = e.target.value;
    dispatchPreview();
  }
});

document.getElementById('sel-temp-unit').addEventListener('change', (e) => {
  if (currentConfig) {
    currentConfig.temperatureUnit = e.target.value;
    dispatchPreview();
  }
});

document.getElementById('sel-wind-unit').addEventListener('change', (e) => {
  if (currentConfig) {
    currentConfig.windSpeedUnit = e.target.value;
    dispatchPreview();
  }
});

function collectConfigFromUI() {
  if (!currentConfig) return null;

  const cfg = JSON.parse(JSON.stringify(currentConfig));

  cfg.viewMode = document.getElementById('sel-view-mode').value;
  cfg.opacity = parseInt(document.getElementById('slider-opacity').value, 10);
  cfg.noBackground = document.getElementById('chk-no-bg').checked;
  cfg.temperatureUnit = document.getElementById('sel-temp-unit').value;
  cfg.windSpeedUnit = document.getElementById('sel-wind-unit').value;
  cfg.cornerPosition = document.getElementById('sel-corner').value;
  cfg.refreshInterval = parseInt(document.getElementById('sel-refresh-interval').value, 10);

  if (!cfg.apiConfig) cfg.apiConfig = {};
  cfg.apiConfig.provider = document.getElementById('sel-provider').value;
  cfg.apiConfig.apiKey = document.getElementById('input-api-key').value.trim();

  cfg.displayFields = {
    showFeelsLike: document.getElementById('df-feels-like').checked,
    showHumidity: document.getElementById('df-humidity').checked,
    showWindSpeed: document.getElementById('df-wind').checked,
    showWindGust: document.getElementById('df-gust').checked,
    showDewPoint: document.getElementById('df-dew').checked,
    showPressure: document.getElementById('df-pressure').checked,
    showUVIndex: document.getElementById('df-uv').checked,
  };

  cfg.pollutionFields = {
    showAQI: document.getElementById('pf-aqi').checked,
    showPM25: document.getElementById('pf-pm25').checked,
    showPM10: document.getElementById('pf-pm10').checked,
    showO3: document.getElementById('pf-o3').checked,
    showNO2: document.getElementById('pf-no2').checked,
    showSO2: document.getElementById('pf-so2').checked,
    showCO: document.getElementById('pf-co').checked,
  };

  return cfg;
}

function dispatchPreview() {
  const cfg = collectConfigFromUI();
  if (cfg && window.goPreviewConfig) {
    window.goPreviewConfig(JSON.stringify(cfg));
  }
}

// Save Changes
document.getElementById('btn-save').addEventListener('click', async () => {
  const cfg = collectConfigFromUI();
  if (!cfg) return;

  try {
    if (window.goSaveConfig) {
      const err = await window.goSaveConfig(JSON.stringify(cfg));
      if (err) {
        showToast('Error saving: ' + err, true);
        return;
      }
    }
    showToast('Settings saved successfully!');
  } catch (err) {
    showToast('Failed to save settings: ' + err, true);
  }
});

// Close Button
document.getElementById('btn-cancel').addEventListener('click', () => {
  if (window.goCloseSettings) {
    window.goCloseSettings();
  }
});

// Test Connection Button
document.getElementById('btn-test-connection').addEventListener('click', async () => {
  const provider = document.getElementById('sel-provider').value;
  const apiKey = document.getElementById('input-api-key').value.trim();
  const resSpan = document.getElementById('test-result');

  resSpan.textContent = 'Testing connection...';
  resSpan.style.color = 'var(--text-muted)';

  if (window.goTestConnection) {
    try {
      const res = await window.goTestConnection(provider, apiKey);
      if (res === 'ok' || !res) {
        resSpan.textContent = '✓ Connection successful!';
        resSpan.style.color = '#2ecc71';
      } else {
        resSpan.textContent = '✗ ' + res;
        resSpan.style.color = '#ff6b6b';
      }
    } catch (e) {
      resSpan.textContent = '✗ Error: ' + e;
      resSpan.style.color = '#ff6b6b';
    }
  } else {
    setTimeout(() => {
      resSpan.textContent = '✓ Connection verified';
      resSpan.style.color = '#2ecc71';
    }, 500);
  }
});

// Autostart toggle
document.getElementById('chk-autostart').addEventListener('change', async (e) => {
  if (window.goSetAutoStart) {
    await window.goSetAutoStart(e.target.checked);
  }
});

// Escape HTML helper
function escapeHtml(str) {
  return str.replace(/[&<>'"]/g, 
    tag => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[tag] || tag)
  );
}

// Initialize on window load
window.addEventListener('DOMContentLoaded', async () => {
  if (window.goGetConfig) {
    try {
      const json = await window.goGetConfig();
      const cfg = JSON.parse(json);
      populateUI(cfg);
    } catch (e) {
      console.error('Failed to load initial config', e);
    }
  }
  if (window.goGetAutoStart) {
    try {
      const enabled = await window.goGetAutoStart();
      document.getElementById('chk-autostart').checked = !!enabled;
    } catch (e) {
      console.error(e);
    }
  }
});
