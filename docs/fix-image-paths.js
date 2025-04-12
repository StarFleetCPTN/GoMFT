// Fix image paths in Markdown files
const fs = require('fs');
const path = require('path');

// Function to recursively find all Markdown files
function findMarkdownFiles(directory) {
  const files = [];
  
  function traverse(dir) {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    
    for (const entry of entries) {
      const fullPath = path.join(dir, entry.name);
      
      if (entry.isDirectory()) {
        traverse(fullPath);
      } else if (entry.isFile() && entry.name.endsWith('.md')) {
        files.push(fullPath);
      }
    }
  }
  
  traverse(directory);
  return files;
}

// Function to fix image paths in a file
function fixImagePaths(filePath) {
  let content = fs.readFileSync(filePath, 'utf8');
  let originalContent = content;
  
  // Replace absolute paths with relative ones based on file location
  const relativeToRoot = path.relative(path.dirname(filePath), path.resolve('static'));
  const relativePath = relativeToRoot.replace(/\\/g, '/');
  
  // Replace any occurrence of ![...](/img/...) with ![...](relativePath/img/...)
  content = content.replace(/!\[(.*?)\]\(\/img\/(.*?)\)/g, 
    (match, alt, imgPath) => `![${alt}](${relativePath}/img/${imgPath})`);
  
  // If content changed, write back to file
  if (content !== originalContent) {
    console.log(`Fixed image paths in ${filePath}`);
    fs.writeFileSync(filePath, content, 'utf8');
    return true;
  }
  
  return false;
}

// Main function
function main() {
  console.log('Finding Markdown files...');
  const docsDir = path.resolve('docs');
  const mdFiles = findMarkdownFiles(docsDir);
  
  console.log(`Found ${mdFiles.length} Markdown files`);
  
  let fixedCount = 0;
  for (const file of mdFiles) {
    if (fixImagePaths(file)) {
      fixedCount++;
    }
  }
  
  console.log(`Fixed image paths in ${fixedCount} files`);
}

main(); 