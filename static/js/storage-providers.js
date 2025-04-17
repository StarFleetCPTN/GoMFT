document.addEventListener('DOMContentLoaded', () => {
  // Show test modal when the test button is clicked
  document.querySelectorAll('.test-provider-btn').forEach(button => {
    button.addEventListener('click', async (e) => {
      e.preventDefault();
      const providerId = button.dataset.providerId;
      const providerName = button.dataset.providerName;
      const testModal = document.getElementById('test-provider-modal');
      
      // Show the modal and add classes to make it visible
      testModal.classList.remove('hidden');
      testModal.classList.add('flex');
      
      // Create loading indicator
      testModal.innerHTML = `
        <div class="relative p-4 w-full max-w-md max-h-full mx-auto">
          <div class="relative bg-white rounded-lg shadow dark:bg-gray-700">
            <div class="flex items-center justify-between p-4 md:p-5 border-b rounded-t dark:border-gray-600">
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                Testing Connection: ${providerName}
              </h3>
              <button type="button" class="close-modal text-gray-400 bg-transparent hover:bg-gray-200 hover:text-gray-900 rounded-lg text-sm w-8 h-8 ms-auto inline-flex justify-center items-center dark:hover:bg-gray-600 dark:hover:text-white">
                <i class="fas fa-times"></i>
                <span class="sr-only">Close modal</span>
              </button>
            </div>
            <div class="p-4 md:p-5" id="test-result">
              <div class="flex items-center justify-center p-8">
                <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500"></div>
                <span class="ml-3">Testing connection...</span>
              </div>
            </div>
          </div>
        </div>
      `;
      
      // Add event listener for close button
      testModal.querySelector('.close-modal').addEventListener('click', () => {
        testModal.classList.add('hidden');
        testModal.classList.remove('flex');
      });
      
      try {
        // Make API call to test the provider
        const response = await fetch(`/storage-providers/${providerId}/test`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          }
        });
        
        // Get the HTML response directly
        const htmlResult = await response.text();
        
        // Replace the test result container with the server-rendered HTML
        document.getElementById('test-result').innerHTML = htmlResult;
      } catch (err) {
        console.error('Error testing provider:', err);
        document.getElementById('test-result').innerHTML = `
          <div class="text-center">
            <i class="fas fa-times-circle text-red-500 text-5xl mb-4"></i>
            <h3 class="mb-2 text-lg font-semibold text-red-500 dark:text-red-400">Connection Failed</h3>
            <p class="text-gray-500 dark:text-gray-400 mb-4">
              Network error: Failed to connect to server
            </p>
          </div>
        `;
      }
    });
  });
  
  // Handle delete confirmation
  document.querySelectorAll('.delete-provider-btn').forEach(button => {
    button.addEventListener('click', (e) => {
      e.preventDefault();
      
      const dialogId = button.dataset.dialogId;
      const providerId = button.dataset.providerId;
      
      // Clone the template
      const templateContent = document.getElementById('delete-dialog-template').content.cloneNode(true);
      const dialog = document.createElement('div');
      dialog.setAttribute('id', dialogId);
      dialog.classList.add('fixed', 'top-0', 'right-0', 'left-0', 'z-50', 'flex', 'justify-center', 'items-center', 'w-full', 'md:inset-0', 'h-[calc(100%-1rem)]', 'max-h-full');
      dialog.appendChild(templateContent);
      document.body.appendChild(dialog);
      
      // Add event listeners
      dialog.querySelector('.delete-confirm-btn').addEventListener('click', async () => {
        try {
          const response = await fetch(`/storage-providers/${providerId}`, {
            method: 'DELETE',
            headers: {
              'Content-Type': 'application/json',
              'X-HTTP-Method-Override': 'DELETE'
            }
          });
          
          if (response.ok) {
            // Reload the page to show updated list
            window.location.reload();
          } else {
            const data = await response.json();
            showToast('error', `Failed to delete provider: ${data.message || response.statusText}`);
            dialog.remove();
          }
        } catch (err) {
          console.error('Error deleting provider:', err);
          showToast('error', `Network error: ${err.message || 'Failed to connect to server'}`);
          dialog.remove();
        }
      });
      
      dialog.querySelector('.cancel-btn').addEventListener('click', () => {
        dialog.remove();
      });
    });
  });
  
  // Helper function to show toast messages
  function showToast(type, message) {
    const toastContainer = document.getElementById('toast-container');
    if (!toastContainer) {
      console.error('Toast container not found');
      return;
    }
    
    const toast = document.createElement('div');
    toast.className = `flex items-center w-full max-w-xs p-4 mb-4 text-gray-500 bg-white rounded-lg shadow dark:text-gray-400 dark:bg-gray-800 ${type === 'error' ? 'border-l-4 border-red-500' : 'border-l-4 border-green-500'}`;
    
    let icon = '';
    if (type === 'error') {
      icon = `<svg class="w-5 h-5 text-red-500" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" fill="currentColor" viewBox="0 0 20 20">
        <path d="M10 .5a9.5 9.5 0 1 0 9.5 9.5A9.51 9.51 0 0 0 10 .5Zm3.5 13H7v-2h6.5v2Zm.5-6.5a1 1 0 0 1-1 1H8a1 1 0 0 1 0-2h5a1 1 0 0 1 1 1Z"/>
      </svg>`;
    } else {
      icon = `<svg class="w-5 h-5 text-green-500" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" fill="currentColor" viewBox="0 0 20 20">
        <path d="M10 .5a9.5 9.5 0 1 0 9.5 9.5A9.51 9.51 0 0 0 10 .5Zm3.707 8.207-4 4a1 1 0 0 1-1.414 0l-2-2a1 1 0 0 1 1.414-1.414L9 10.586l3.293-3.293a1 1 0 0 1 1.414 1.414Z"/>
      </svg>`;
    }
    
    toast.innerHTML = `
      <div class="inline-flex items-center justify-center flex-shrink-0 w-8 h-8 text-${type === 'error' ? 'red' : 'green'}-500 bg-${type === 'error' ? 'red' : 'green'}-100 rounded-lg dark:bg-${type === 'error' ? 'red' : 'green'}-800 dark:text-${type === 'error' ? 'red' : 'green'}-200">
        ${icon}
      </div>
      <div class="ml-3 text-sm font-normal">${message}</div>
      <button type="button" class="ml-auto -mx-1.5 -my-1.5 bg-white text-gray-400 hover:text-gray-900 rounded-lg focus:ring-2 focus:ring-gray-300 p-1.5 hover:bg-gray-100 inline-flex items-center justify-center h-8 w-8 dark:text-gray-500 dark:hover:text-white dark:bg-gray-800 dark:hover:bg-gray-700" aria-label="Close">
        <span class="sr-only">Close</span>
        <svg class="w-3 h-3" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 14 14">
          <path stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="m1 1 6 6m0 0 6 6M7 7l6-6M7 7l-6 6"/>
        </svg>
      </button>
    `;
    
    toastContainer.appendChild(toast);
    
    // Remove toast after 5 seconds
    setTimeout(() => {
      toast.remove();
    }, 5000);
    
    // Handle close button
    const closeButton = toast.querySelector('button');
    closeButton.addEventListener('click', () => {
      toast.remove();
    });
  }
}); 