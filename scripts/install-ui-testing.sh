#!/bin/bash

# Script to install UI testing dependencies

echo "Installing Node.js dependencies for UI testing..."
npm install

echo "Installing Playwright browsers..."
npx playwright install

echo "UI testing setup complete!"
echo ""
echo "You can now run UI tests with the following commands:"
echo "  npm test             - Run all tests"
echo "  npm run test:headed  - Run tests with visible browsers"
echo "  npm run test:debug   - Run tests in debug mode"
echo "  npm run test:ui      - Run tests with Playwright UI"
echo ""
echo "For more information, see tests/README.md" 