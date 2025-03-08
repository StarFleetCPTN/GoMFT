// Theme management for GoMFT application
document.addEventListener('DOMContentLoaded', function() {
  initializeTheme();
});

// Initialize theme based on user preference
function initializeTheme() {
  const storedTheme = getCookie('theme');
  
  if (storedTheme === 'dark') {
    applyDarkTheme();
  } else if (storedTheme === 'system') {
    applySystemTheme();
  } else {
    // Default to light theme
    applyLightTheme();
  }
  
  // Listen for theme changes from system
  if (window.matchMedia) {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    
    // Add change listener
    try {
      // Chrome & Firefox
      mediaQuery.addEventListener('change', (e) => {
        if (getCookie('theme') === 'system') {
          e.matches ? applyDarkTheme(false) : applyLightTheme(false);
        }
      });
    } catch (e1) {
      try {
        // Safari
        mediaQuery.addListener((e) => {
          if (getCookie('theme') === 'system') {
            e.matches ? applyDarkTheme(false) : applyLightTheme(false);
          }
        });
      } catch (e2) {
        console.error('Could not add media query listener', e2);
      }
    }
  }
  
  // Listen for theme changes via HTMX
  document.body.addEventListener('htmx:afterRequest', function(event) {
    if (event.detail.requestConfig && event.detail.requestConfig.path === '/profile/theme') {
      // Refresh the theme after update
      const updatedTheme = getCookie('theme');
      applyTheme(updatedTheme);
    }
  });
}

// Toggle between light and dark theme
function toggleTheme() {
  const currentTheme = document.documentElement.classList.contains('dark') ? 'dark' : 'light';
  if (currentTheme === 'dark') {
    applyLightTheme();
    setCookie('theme', 'light', 365);
  } else {
    applyDarkTheme();
    setCookie('theme', 'dark', 365);
  }
  
  // Add a subtle animation effect
  document.body.classList.add('theme-transition');
  setTimeout(() => {
    document.body.classList.remove('theme-transition');
  }, 500);
}

// Apply theme based on theme name
function applyTheme(theme) {
  if (theme === 'dark') {
    applyDarkTheme();
  } else if (theme === 'system') {
    applySystemTheme();
  } else {
    applyLightTheme();
  }
}

// Apply dark theme
function applyDarkTheme(setClass = true) {
  if (setClass) {
    document.documentElement.classList.add('dark');
    document.documentElement.classList.remove('light');
  }
  document.documentElement.style.colorScheme = 'dark';
  
  // Add transition for smooth theme switching
  document.documentElement.style.transition = 'background-color 0.3s ease, color 0.3s ease';
  
  updateThemeColors('dark');
  
  // Update theme toggle icon
  updateThemeToggleIcon('dark');
  
  // Store user preference in localStorage as a backup
  localStorage.setItem('theme', 'dark');
}

// Apply light theme
function applyLightTheme(setClass = true) {
  if (setClass) {
    document.documentElement.classList.remove('dark');
    document.documentElement.classList.add('light');
  }
  document.documentElement.style.colorScheme = 'light';
  
  // Add transition for smooth theme switching
  document.documentElement.style.transition = 'background-color 0.3s ease, color 0.3s ease';
  
  updateThemeColors('light');
  
  // Update theme toggle icon
  updateThemeToggleIcon('light');
  
  // Store user preference in localStorage as a backup
  localStorage.setItem('theme', 'light');
}

// Apply system theme based on user's OS preference
function applySystemTheme() {
  if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
    applyDarkTheme(true);
  } else {
    applyLightTheme(true);
  }
  
  // Store user preference in localStorage as a backup
  localStorage.setItem('theme', 'system');
}

// Update theme colors
function updateThemeColors(theme) {
  // This function can be expanded to update specific UI elements
  // that might need special handling beyond CSS classes
  
  // For example, updating charts, custom components, etc.
  if (theme === 'dark') {
    // Apply dark theme specific changes
    // Ensure better contrast for text elements
    const textElements = document.querySelectorAll('.text-gray-700, .text-gray-800, .text-gray-900, .text-secondary-700, .text-secondary-800, .text-secondary-900');
    textElements.forEach(el => {
      if (!el.classList.contains('dark:text-white') && 
          !el.classList.contains('dark:text-gray-100') && 
          !el.classList.contains('dark:text-gray-200') &&
          !el.classList.contains('dark:text-secondary-100') && 
          !el.classList.contains('dark:text-secondary-200')) {
        el.classList.add('dark:text-secondary-200');
      }
    });
    
    // Ensure better contrast for background elements
    const bgElements = document.querySelectorAll('.bg-gray-800, .bg-gray-900, .bg-secondary-800, .bg-secondary-900');
    bgElements.forEach(el => {
      if (!el.classList.contains('dark:bg-gray-700') &&
          !el.classList.contains('dark:bg-secondary-700')) {
        el.classList.add('dark:bg-secondary-700');
      }
    });
    
    // Apply custom animations for dark mode
    document.body.classList.add('theme-dark-animation');
    setTimeout(() => {
      document.body.classList.remove('theme-dark-animation');
    }, 500);
  } else {
    // Apply light theme specific changes
    
    // Apply custom animations for light mode
    document.body.classList.add('theme-light-animation');
    setTimeout(() => {
      document.body.classList.remove('theme-light-animation');
    }, 500);
  }
}

// Update theme toggle icon
function updateThemeToggleIcon(theme) {
  const themeToggle = document.getElementById('theme-toggle');
  if (!themeToggle) return;
  
  const sunIcon = themeToggle.querySelector('.fa-sun');
  const moonIcon = themeToggle.querySelector('.fa-moon');
  
  if (theme === 'dark') {
    if (sunIcon) sunIcon.classList.remove('hidden');
    if (moonIcon) moonIcon.classList.add('hidden');
  } else {
    if (sunIcon) sunIcon.classList.add('hidden');
    if (moonIcon) moonIcon.classList.remove('hidden');
  }
}

// Helper function to get cookie value
function getCookie(name) {
  const value = `; ${document.cookie}`;
  const parts = value.split(`; ${name}=`);
  if (parts.length === 2) return parts.pop().split(';').shift();
  
  // Fallback to localStorage if cookie is not available
  return localStorage.getItem(name) || '';
}

// Helper function to set cookie
function setCookie(name, value, days) {
  let expires = '';
  if (days) {
    const date = new Date();
    date.setTime(date.getTime() + (days * 24 * 60 * 60 * 1000));
    expires = '; expires=' + date.toUTCString();
  }
  document.cookie = name + '=' + (value || '') + expires + '; path=/; SameSite=Strict';
}

// Add CSS for theme transition animations
const style = document.createElement('style');
style.textContent = `
  .theme-transition {
    transition: background-color 0.3s ease, color 0.3s ease, border-color 0.3s ease, box-shadow 0.3s ease;
  }
  
  .theme-dark-animation {
    animation: darkModeIn 0.5s ease forwards;
  }
  
  .theme-light-animation {
    animation: lightModeIn 0.5s ease forwards;
  }
  
  @keyframes darkModeIn {
    0% { opacity: 0.8; }
    100% { opacity: 1; }
  }
  
  @keyframes lightModeIn {
    0% { opacity: 0.8; }
    100% { opacity: 1; }
  }
`;
document.head.appendChild(style);
