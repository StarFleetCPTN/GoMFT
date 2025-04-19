// Import HTMX
import 'htmx.org';

// Import Alpine.js
import Alpine from 'alpinejs';
window.Alpine = Alpine;

// Import Flowbite
import 'flowbite';
import 'flowbite/dist/flowbite.css';

// Import FullCalendar
import { Calendar } from '@fullcalendar/core';
import dayGridPlugin from '@fullcalendar/daygrid';
import timeGridPlugin from '@fullcalendar/timegrid';
import listPlugin from '@fullcalendar/list';
import interactionPlugin from '@fullcalendar/interaction';

// Make FullCalendar available globally in the format expected by the calendar template
window.FullCalendar = {
  Calendar: Calendar,  // This makes FullCalendar.Calendar a constructor
  dayGridPlugin: dayGridPlugin,
  timeGridPlugin: timeGridPlugin,
  listPlugin: listPlugin,
  interactionPlugin: interactionPlugin
};

// Import Popper.js
import * as Popper from '@popperjs/core';
window.Popper = Popper;

// Import Tippy.js
import tippy from 'tippy.js';
import 'tippy.js/dist/tippy.css';
import 'tippy.js/themes/light.css';
window.tippy = tippy;

// Initialize Flowbite components
document.addEventListener('DOMContentLoaded', () => {
    // Initialize Alpine.js
    Alpine.start();
    
    // Initialize Flowbite modals
    const modalTriggers = document.querySelectorAll('[data-modal-target]');
    modalTriggers.forEach(trigger => {
        const targetId = trigger.getAttribute('data-modal-target');
        const targetModal = document.getElementById(targetId);
        if (targetModal) {
            // Show modal
            trigger.addEventListener('click', () => {
                targetModal.classList.remove('hidden');
                targetModal.classList.add('flex');
                // Add backdrop
                document.body.style.overflow = 'hidden';
            });
            
            // Handle modal hide buttons
            const hideButtons = targetModal.querySelectorAll('[data-modal-hide]');
            hideButtons.forEach(button => {
                button.addEventListener('click', () => {
                    targetModal.classList.add('hidden');
                    targetModal.classList.remove('flex');
                    // Remove backdrop
                    document.body.style.overflow = '';
                });
            });

            // Handle clicking outside the modal
            targetModal.addEventListener('click', (event) => {
                if (event.target === targetModal || event.target.classList.contains('fixed')) {
                    targetModal.classList.add('hidden');
                    targetModal.classList.remove('flex');
                    // Remove backdrop
                    document.body.style.overflow = '';
                }
            });
        }
    });
}); 