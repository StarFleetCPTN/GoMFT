// Add script to ensure dark mode is properly applied
document.addEventListener('DOMContentLoaded', function() {
    // Apply body dark class when theme changes
    const isDark = document.documentElement.classList.contains('dark');
    if (isDark) {
        document.body.classList.add('dark');
        
        // Also apply to containers
        const jobsContainer = document.getElementById('jobs-container');
        const configsContainer = document.getElementById('configs-container');
        
        if (jobsContainer) jobsContainer.classList.add('dark');
        if (configsContainer) configsContainer.classList.add('dark');
    }

    // Initialize admin dropdown toggle if available
    const adminDropdownToggle = document.querySelector('[data-collapse-toggle="dropdown-settings"]');
    const adminDropdown = document.getElementById('dropdown-settings');
    
    if (adminDropdownToggle && adminDropdown) {
        // Check if we should show the dropdown (if current page is under admin section)
        const currentPath = window.location.pathname;
        if (currentPath.startsWith('/admin')) {
            adminDropdown.classList.remove('hidden');
        }

        // Add click event listener
        adminDropdownToggle.addEventListener('click', function() {
            adminDropdown.classList.toggle('hidden');
        });
    }
}); 