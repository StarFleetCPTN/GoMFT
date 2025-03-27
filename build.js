import * as esbuild from 'esbuild';
import path from 'path';
import { fileURLToPath } from 'url';
import { execSync } from 'child_process';
import fs from 'fs';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const isWatch = process.argv.includes('--watch');

// Ensure the dist directory exists
const distDir = path.join(__dirname, 'static', 'dist');
if (!fs.existsSync(distDir)) {
  fs.mkdirSync(distDir, { recursive: true });
}

// Copy Font Awesome files
const fontAwesomeSrcDir = path.join(__dirname, 'node_modules', '@fortawesome', 'fontawesome-free');
const fontAwesomeDestDir = path.join(distDir, 'fontawesome');

// Copy CSS files
const cssFiles = [
  'css/all.min.css',
  'css/fontawesome.min.css',
  'css/solid.min.css',
  'css/regular.min.css',
  'css/brands.min.css'
];

cssFiles.forEach(file => {
  const srcFile = path.join(fontAwesomeSrcDir, file);
  const destFile = path.join(fontAwesomeDestDir, file);
  const destDir = path.dirname(destFile);
  
  if (!fs.existsSync(destDir)) {
    fs.mkdirSync(destDir, { recursive: true });
  }
  
  if (fs.existsSync(srcFile)) {
    fs.copyFileSync(srcFile, destFile);
  }
});

// Copy webfonts
const webfontsSrcDir = path.join(fontAwesomeSrcDir, 'webfonts');
const webfontsDestDir = path.join(fontAwesomeDestDir, 'webfonts');

if (!fs.existsSync(webfontsDestDir)) {
  fs.mkdirSync(webfontsDestDir, { recursive: true });
}

fs.readdirSync(webfontsSrcDir).forEach(file => {
  fs.copyFileSync(
    path.join(webfontsSrcDir, file),
    path.join(webfontsDestDir, file)
  );
});

const commonConfig = {
  sourcemap: true,
  minify: true,
  bundle: true,
  platform: 'browser',
  target: ['es2020'],
};

async function buildTailwind() {
  console.log('Building Tailwind CSS...');
  execSync('npx tailwindcss -i ./static/css/app.css -o ./static/dist/app.css --minify');
}

async function build() {
  try {
    // Build vendor JavaScript bundle (CDN dependencies)
    await esbuild.build({
      ...commonConfig,
      entryPoints: ['static/js/vendor.js'],
      outfile: 'static/dist/vendor.js',
      format: 'iife',
    });

    // Build application JavaScript
    await esbuild.build({
      ...commonConfig,
      entryPoints: ['static/js/app.js'],
      outfile: 'static/dist/app.js',
      format: 'iife',
    });

    // Build initialization JavaScript
    await esbuild.build({
      ...commonConfig,
      entryPoints: ['static/js/init.js'],
      outfile: 'static/dist/init.js',
      format: 'iife',
    });

    // Build CSS with Tailwind
    await buildTailwind();

    console.log('Build completed successfully!');
  } catch (error) {
    console.error('Build failed:', error);
    process.exit(1);
  }
}

if (isWatch) {
  // Watch mode
  console.log('Starting watch mode...');
  const ctx = await esbuild.context(commonConfig);
  await ctx.watch();
  
  // Watch Tailwind CSS
  execSync('npx tailwindcss -i ./static/css/app.css -o ./static/dist/app.css --watch');
  
  console.log('Watching for changes...');
} else {
  // Single build
  build();
} 