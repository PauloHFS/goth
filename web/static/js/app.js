/**
 * GOTH Stack - Minimal JavaScript
 *
 * HTMX philosophy: minimal client-side JS.
 * All state and logic lives on the server.
 */

(function () {
  "use strict";

  // Initialize Lucide icons
  function initIcons() {
    if (typeof lucide !== "undefined") {
      lucide.createIcons();
    }
  }

  // Theme: light/dark/system mode
  function updateThemeIcons(theme) {
    var sunIcon = document.querySelector(".sun-icon");
    var moonIcon = document.querySelector(".moon-icon");
    var systemIcon = document.querySelector(".system-icon");
    
    if (!sunIcon || !moonIcon || !systemIcon) return;
    
    // Hide all icons first
    sunIcon.classList.add("hidden");
    moonIcon.classList.add("hidden");
    systemIcon.classList.add("hidden");
    sunIcon.classList.remove("block");
    moonIcon.classList.remove("block");
    systemIcon.classList.remove("block");
    
    // Show appropriate icon based on selected theme
    if (theme === "light") {
      sunIcon.classList.remove("hidden");
      sunIcon.classList.add("block");
    } else if (theme === "dark") {
      moonIcon.classList.remove("hidden");
      moonIcon.classList.add("block");
    } else if (theme === "system") {
      systemIcon.classList.remove("hidden");
      systemIcon.classList.add("block");
    }
  }

  function initTheme() {
    var saved = localStorage.getItem("theme");
    var prefersDark = window.matchMedia(
      "(prefers-color-scheme: dark)",
    ).matches;
    
    // Default to 'system' if no preference saved
    var theme = saved || "system";
    
    // Apply theme based on selection
    function applyTheme(selectedTheme) {
      if (selectedTheme === "system") {
        if (prefersDark) {
          document.documentElement.classList.add("dark");
        } else {
          document.documentElement.classList.remove("dark");
        }
      } else if (selectedTheme === "dark") {
        document.documentElement.classList.add("dark");
      } else {
        document.documentElement.classList.remove("dark");
      }
    }
    
    applyTheme(theme);
    window.__theme = theme;
    updateThemeIcons(theme);
  }

  function toggleTheme() {
    var themes = ["light", "dark", "system"];
    var currentTheme = localStorage.getItem("theme") || "system";
    var currentIndex = themes.indexOf(currentTheme);
    var nextTheme = themes[(currentIndex + 1) % themes.length];
    
    localStorage.setItem("theme", nextTheme);
    window.__theme = nextTheme;
    
    // Apply the new theme
    var prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    
    if (nextTheme === "system") {
      if (prefersDark) {
        document.documentElement.classList.add("dark");
      } else {
        document.documentElement.classList.remove("dark");
      }
    } else if (nextTheme === "dark") {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
    
    // Update icons
    updateThemeIcons(nextTheme);

    // Dispatch custom event for other components to react
    window.dispatchEvent(new CustomEvent("themechange", { detail: { theme: nextTheme } }));
  }

  // SVG icons for toasts (inline - no dependency)
  var icons = {
    success:
      '<svg class="h-6 w-6 text-green-400" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" /></svg>',
    error:
      '<svg class="h-6 w-6 text-red-400" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" /></svg>',
    warning:
      '<svg class="h-6 w-6 text-yellow-400" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" /></svg>',
    info:
      '<svg class="h-6 w-6 text-blue-400" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd" /></svg>',
  };

  // Toast notifications
  function showToast(type, title, message, duration) {
    if (duration === void 0) {
      duration = 5000;
    }

    var container = document.getElementById("toast-container");
    if (!container) return;

    var colors = {
      success: "green",
      error: "red",
      warning: "yellow",
      info: "blue",
    };

    var icon = icons[type] || icons.info;
    var color = colors[type] || "blue";

    var toast = document.createElement("div");
    toast.className =
      "pointer-events-auto w-full max-w-sm overflow-hidden rounded-lg bg-white shadow-lg ring-1 ring-black ring-opacity-5 transition-all duration-300 ease-in-out";
    toast.innerHTML =
      '<div class="p-4"><div class="flex items-start"><div class="flex-shrink-0">' +
      icon +
      '</div><div class="ml-3 w-0 flex-1 pt-0.5"><p class="text-sm font-medium text-gray-900">' +
      title +
      '</p><p class="mt-1 text-sm text-gray-500">' +
      message +
      '</p></div><div class="ml-4 flex flex-shrink-0"><button type="button" class="inline-flex rounded-md bg-white text-gray-400 hover:text-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2" onclick="this.parentElement.parentElement.parentElement.remove()"><span class="sr-only">Close</span><svg class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" /></svg></button></div></div></div>';

    container.querySelector("div").appendChild(toast);

    setTimeout(function () {
      if (toast.parentElement) {
        toast.remove();
      }
    }, duration);
  }

  // Check for toast from server (via URL hash or session)
  function checkServerToast() {
    var hash = window.location.hash;
    if (hash.indexOf("toast=") === 0) {
      try {
        var data = JSON.parse(decodeURIComponent(hash.substring(6)));
        showToast(data.type, data.title, data.message, data.duration);
        window.history.replaceState(
          null,
          "",
          window.location.pathname + window.location.search,
        );
      } catch (e) {
        // Invalid toast data, ignore
      }
    }
  }

  // Expose globally
  window.toggleTheme = toggleTheme;
  window.showToast = showToast;

  // Initialize on DOM ready
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () {
      initTheme();
      checkServerToast();
      initIcons();

      // Bind theme toggle
      var btn = document.getElementById("theme-toggle");
      if (btn) {
        btn.addEventListener("click", toggleTheme);
      }
    });
  } else {
    initTheme();
    checkServerToast();
    initIcons();

    var btn = document.getElementById("theme-toggle");
    if (btn) {
      btn.addEventListener("click", toggleTheme);
    }
  }

  // HTMX: initialize icons after swap
  document.body.addEventListener("htmx:afterSwap", function () {
    initIcons();
  });

  // HTMX: show toast from response header
  document.body.addEventListener("htmx:afterRequest", function (event) {
    var toastHeader = event.detail.xhr.getResponseHeader("X-Toast");
    if (toastHeader) {
      try {
        var data = JSON.parse(toastHeader);
        showToast(data.type, data.title, data.message, data.duration || 5000);
      } catch (e) {
        console.error("Failed to parse toast header:", e);
      }
    }
  });
})();
