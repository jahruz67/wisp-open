// Wisp (WIS Free V3) Website Interactive Controller

document.addEventListener('DOMContentLoaded', () => {
  initHeader();
  initDictationSimulator();
  initModelTabs();
  initDistroSwitcher();
  initCopyButtons();
  initScrollReveal();
  initCurrentYear();
});

/* 1. Header Navigation & Mobile Menu */
function initHeader() {
  const header = document.querySelector('.site-header');
  const toggle = document.querySelector('.nav-toggle');
  const navLinks = document.querySelector('.nav-links');

  if (header) {
    window.addEventListener('scroll', () => {
      header.classList.toggle('scrolled', window.scrollY > 15);
    }, { passive: true });
  }

  if (toggle && navLinks) {
    toggle.addEventListener('click', () => {
      const isOpen = navLinks.classList.toggle('open');
      toggle.setAttribute('aria-expanded', String(isOpen));
      toggle.setAttribute('aria-label', isOpen ? 'Close navigation' : 'Open navigation');
    });

    navLinks.querySelectorAll('a').forEach(link => {
      link.addEventListener('click', () => {
        navLinks.classList.remove('open');
        toggle.setAttribute('aria-expanded', 'false');
      });
    });

    document.addEventListener('click', (e) => {
      if (!header.contains(e.target) && navLinks.classList.contains('open')) {
        navLinks.classList.remove('open');
        toggle.setAttribute('aria-expanded', 'false');
      }
    });
  }
}

/* 2. Interactive Dictation & Floating HUD Overlay Simulator */
function initDictationSimulator() {
  const overlay = document.getElementById('hudOverlay');
  const hudIcon = document.getElementById('hudIcon');
  const hudTitle = document.getElementById('hudTitle');
  const hudSub = document.getElementById('hudSub');
  const hudTimer = document.getElementById('hudTimer');
  const waveBars = document.querySelectorAll('.hud-waveform .wave-bar');
  const typedContainer = document.getElementById('typedText');
  const editorTarget = document.getElementById('editorTarget');
  const editorMeta = document.getElementById('editorMeta');
  const simulateBtn = document.getElementById('simulateBtn');
  const presetChips = document.querySelectorAll('.preset-chip');

  if (!overlay || !typedContainer) return;

  const presets = [
    {
      id: 'flow',
      label: 'Everyday Flow',
      target: 'Slack · #product-design',
      meta: 'Groq Whisper Turbo · 0.8s',
      spoken: 'Let’s move the product review to Thursday afternoon and invite the engineering leads too.',
      refined: 'Let’s move the product review to Thursday afternoon and invite the engineering leads too.'
    },
    {
      id: 'code',
      label: 'Code & Terminal',
      target: 'VS Code · speech_engine.py',
      meta: 'Local Parakeet TDT v3 (CPU) · Refined',
      spoken: 'define a function called record and transcribe with timeout equals five seconds and error handling',
      refined: 'def record_and_transcribe(timeout: int = 5) -> str:\n    try:\n        return engine.transcribe(stream_audio(), timeout=timeout)\n    except AudioError as err:\n        logger.warning(f"Audio capture failed: {err}")\n        return ""'
    },
    {
      id: 'notes',
      label: 'Meeting Memo',
      target: 'Obsidian · Sprint 14 Notes',
      meta: '100% Offline Whisper (Vulkan GPU)',
      spoken: 'WIS Free V3 bundles ydotool for native Wayland direct typing with zero clipboard interference',
      refined: '- **Input stack**: WIS Free V3 integrates `ydotoold` for universal direct typing on Linux Wayland/X11.\n- **Privacy**: Local Whisper and Parakeet models execute entirely on-device without cloud telemetry.'
    },
    {
      id: 'email',
      label: 'Email Reply',
      target: 'Thunderbird · Re: Roadmap sync',
      meta: 'Groq + Llama 3.3 Versatile',
      spoken: 'thanks for the update looks great to me go ahead and cut the 1.67 release tomorrow morning',
      refined: 'Hi team,\n\nThanks for the update — looks great to me. Let’s proceed with cutting the v1.67 release tomorrow morning.'
    }
  ];

  let currentPresetIndex = 0;
  let isSimulating = false;
  let waveAnimInterval = null;
  let timerInterval = null;
  let typingTimeout = null;

  function setWaveBars(active) {
    if (waveAnimInterval) clearInterval(waveAnimInterval);
    if (!active) {
      waveBars.forEach(bar => {
        bar.style.height = '6px';
        bar.style.opacity = '0.5';
      });
      return;
    }

    waveAnimInterval = setInterval(() => {
      waveBars.forEach((bar, idx) => {
        // Natural speech modulation
        const base = Math.sin((Date.now() / 150) + idx * 0.4);
        const noise = Math.random() * 0.4;
        const normalized = Math.max(0.15, Math.min(1, Math.abs(base) * 0.7 + noise));
        bar.style.height = `${Math.round(normalized * 24 + 4)}px`;
        bar.style.opacity = '1';
      });
    }, 80);
  }

  function runSimulation(presetIndex) {
    if (isSimulating) {
      clearTimeout(typingTimeout);
      clearInterval(waveAnimInterval);
      clearInterval(timerInterval);
    }
    isSimulating = true;

    const data = presets[presetIndex];
    if (editorTarget) editorTarget.textContent = data.target;
    if (editorMeta) editorMeta.textContent = data.meta;
    typedContainer.textContent = '';

    // Step 1: RECORDING STATE
    overlay.className = 'hud-overlay is-recording';
    hudIcon.className = 'hud-mic-indicator recording';
    hudIcon.innerHTML = `<img src="./tray_recording.png" alt="Recording" width="14" height="14">`;
    hudTitle.textContent = 'Listening...';
    hudSub.textContent = 'Hold Ctrl+Super';
    hudTimer.textContent = '00:01';
    setWaveBars(true);

    let seconds = 1;
    timerInterval = setInterval(() => {
      seconds++;
      hudTimer.textContent = `00:0${seconds}`;
    }, 1000);

    // After 2.4s of simulated speech, trigger TRANSCRIBING
    typingTimeout = setTimeout(() => {
      clearInterval(timerInterval);
      clearInterval(waveAnimInterval);
      setWaveBars(false);

      // Step 2: TRANSCRIBING STATE
      overlay.className = 'hud-overlay is-transcribing';
      hudIcon.className = 'hud-mic-indicator transcribing';
      hudIcon.innerHTML = `<img src="./tray_transcribing.png" alt="Transcribing" width="14" height="14">`;
      hudTitle.textContent = 'Transcribing...';
      hudSub.textContent = data.meta.includes('Local') ? 'Local whisper.cpp' : 'Groq Whisper Turbo';
      hudTimer.textContent = '0.4s';

      // Step 3: STREAM TYPING INTO EDITOR
      setTimeout(() => {
        overlay.className = 'hud-overlay is-refined';
        hudIcon.className = 'hud-mic-indicator';
        hudIcon.innerHTML = `<img src="./tray_idle.png" alt="Ready" width="14" height="14" style="opacity: 0.85;">`;
        hudTitle.textContent = 'Direct Typing';
        hudSub.textContent = 'Auto-pasting into active app';

        let charIndex = 0;
        const textToType = data.refined;
        
        function typeNextChar() {
          if (charIndex < textToType.length) {
            typedContainer.textContent += textToType.charAt(charIndex);
            charIndex++;
            const delay = textToType.charAt(charIndex - 1) === '\n' ? 90 : 18;
            typingTimeout = setTimeout(typeNextChar, delay);
          } else {
            // Finished typing!
            setTimeout(() => {
              overlay.className = 'hud-overlay';
              hudTitle.textContent = 'Ready';
              hudSub.textContent = 'Hold Ctrl+Super to speak';
              isSimulating = false;
            }, 1200);
          }
        }
        typeNextChar();

      }, 700);

    }, 2400);
  }

  // Preset switching
  presetChips.forEach((chip, index) => {
    chip.addEventListener('click', () => {
      presetChips.forEach(c => c.classList.remove('active'));
      chip.classList.add('active');
      currentPresetIndex = index;
      runSimulation(currentPresetIndex);
    });
  });

  if (simulateBtn) {
    simulateBtn.addEventListener('click', () => {
      runSimulation(currentPresetIndex);
    });
  }

  // Trigger initial demo on load after a brief polite pause
  setTimeout(() => {
    runSimulation(0);
  }, 900);
}

/* 3. Interactive Engine & Model Tabs */
function initModelTabs() {
  const tabs = document.querySelectorAll('.model-tab-btn');
  const rows = document.querySelectorAll('.models-table tbody tr');

  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      tabs.forEach(t => t.classList.remove('active'));
      tab.classList.add('active');

      const filter = tab.dataset.filter;

      rows.forEach(row => {
        const category = row.dataset.category;
        if (filter === 'all' || category === filter) {
          row.style.display = '';
        } else {
          row.style.display = 'none';
        }
      });
    });
  });
}

/* 4. Linux Distro Switcher */
function initDistroSwitcher() {
  const tabs = document.querySelectorAll('.distro-tab');
  const terminalText = document.getElementById('linuxTerminalCommand');
  const copyBtn = document.getElementById('linuxCopyBtn');

  const commands = {
    deb: 'sudo apt install ./wis-free-v3_1.67.4-1_amd64.deb',
    rpm: 'sudo dnf install ./wis-free-v3-1.67.4-1.x86_64.rpm',
    zypper: 'sudo zypper install ./wis-free-v3-1.67.4-1.x86_64.rpm',
    source: 'bash scripts/build-linux.sh --install-user'
  };

  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      tabs.forEach(t => t.classList.remove('active'));
      tab.classList.add('active');

      const distro = tab.dataset.distro;
      if (commands[distro] && terminalText && copyBtn) {
        terminalText.textContent = commands[distro];
        copyBtn.dataset.copy = commands[distro];
      }
    });
  });
}

/* 5. 1-Click Clipboard Copy Buttons */
function initCopyButtons() {
  const copyButtons = document.querySelectorAll('.copy-btn');

  copyButtons.forEach(btn => {
    btn.addEventListener('click', async () => {
      const textToCopy = btn.dataset.copy || btn.closest('.terminal-box')?.querySelector('.terminal-code')?.textContent?.trim();
      if (!textToCopy) return;

      try {
        await navigator.clipboard.writeText(textToCopy);
        const originalText = btn.innerHTML;
        btn.classList.add('copied');
        btn.innerHTML = `<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg> Copied!`;
        
        setTimeout(() => {
          btn.classList.remove('copied');
          btn.innerHTML = originalText;
        }, 2000);
      } catch (err) {
        console.warn('Clipboard write failed:', err);
      }
    });
  });
}

/* 6. IntersectionObserver Scroll Animations */
function initScrollReveal() {
  const elements = document.querySelectorAll('.reveal');
  if (!elements.length) return;

  const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        entry.target.classList.add('visible');
        observer.unobserve(entry.target);
      }
    });
  }, { threshold: 0.1 });

  elements.forEach((el, index) => {
    el.style.transitionDelay = `${(index % 3) * 70}ms`;
    observer.observe(el);
  });
}

/* 7. Footer Current Year */
function initCurrentYear() {
  const yearElem = document.getElementById('year');
  if (yearElem) {
    yearElem.textContent = new Date().getFullYear();
  }
}
