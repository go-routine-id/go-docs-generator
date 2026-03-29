package docs

// docsTemplate is the HTML template for documentation
const docsTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Info.Title}}</title>
    <script src="https://unpkg.com/react@18/umd/react.development.js" crossorigin></script>
    <script src="https://unpkg.com/react-dom@18/umd/react-dom.development.js" crossorigin></script>
    <script src="https://unpkg.com/@babel/standalone/babel.min.js"></script>
    <script src="https://unpkg.com/reactflow@11/dist/umd/index.js"></script>
    <link rel="stylesheet" href="https://unpkg.com/reactflow@11/dist/style.css" />
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=Fira+Code:wght@400;500&display=swap" rel="stylesheet">
    <style>
        :root {
            /* Google Colors */
            --google-blue: #1a73e8;
            --google-blue-dark: #1557b0;
            --google-blue-light: #e8f0fe;
            --google-red: #ea4335;
            --google-yellow: #fbbc05;
            --google-green: #34a853;
            --google-gray: #5f6368;
            --google-gray-light: #dadce0;

            /* Theme - Google Light */
            --primary: var(--google-blue);
            --primary-dark: var(--google-blue-dark);
            --primary-light: var(--google-blue-light);
            --secondary: #00acc1;
            --success: var(--google-green);
            --warning: var(--google-yellow);
            --danger: var(--google-red);

            /* Light Theme Backgrounds */
            --bg: #ffffff;
            --bg-secondary: #f8f9fa;
            --bg-tertiary: #f1f3f4;

            /* Card & Container */
            --card-bg: #ffffff;
            --card-shadow: 0 1px 2px 0 rgba(60,64,67,0.3), 0 1px 3px 1px rgba(60,64,67,0.15);
            --card-shadow-hover: 0 1px 3px 0 rgba(60,64,67,0.3), 0 4px 8px 3px rgba(60,64,67,0.15);

            /* Text Colors */
            --text: #202124;
            --text-secondary: #5f6368;
            --text-muted: #80868b;

            /* Borders */
            --border: #dadce0;
            --border-light: #e8eaed;

            /* Sidebar */
            --sidebar-width: 280px;
            --sidebar-bg: #ffffff;
            --sidebar-hover: var(--bg-tertiary);
            --sidebar-active: var(--primary-light);
            --sidebar-text: var(--text-secondary);
            --sidebar-text-active: var(--primary);
            --sidebar-border: var(--border);
        }

        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: 'Inter', sans-serif;
            background: var(--bg);
            color: var(--text);
            line-height: 1.6;
        }

        /* Mobile Header */
        .mobile-header {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            height: 64px;
            background: var(--bg);
            border-bottom: 1px solid var(--border);
            padding: 0 1rem;
            align-items: center;
            gap: 1rem;
            z-index: 300;
            box-shadow: 0 1px 2px 0 rgba(60,64,67,0.1);
        }

        .hamburger {
            width: 48px;
            height: 48px;
            border: none;
            background: none;
            cursor: pointer;
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: center;
            gap: 5px;
            border-radius: 50%;
            transition: background 0.2s;
        }

        .hamburger:hover {
            background: var(--bg-tertiary);
        }

        .hamburger span {
            width: 18px;
            height: 2px;
            background: var(--text);
            transition: all 0.3s;
        }

        .mobile-title {
            font-weight: 500;
            font-size: 1.25rem;
            color: var(--text);
        }

        /* Sidebar Overlay */
        .sidebar-overlay {
            display: none;
            position: fixed;
            inset: 0;
            background: rgba(32,33,36,0.6);
            z-index: 250;
            opacity: 0;
            pointer-events: none;
            transition: opacity 0.3s;
        }

        .sidebar-overlay.visible {
            opacity: 1;
            pointer-events: auto;
        }

        /* Sidebar */
        .sidebar {
            position: fixed;
            left: 0;
            top: 0;
            bottom: 0;
            width: var(--sidebar-width);
            background: var(--sidebar-bg);
            border-right: 1px solid var(--border);
            display: flex;
            flex-direction: column;
            z-index: 260;
            overflow: hidden;
        }

        .sidebar-header {
            padding: 1.5rem;
            border-bottom: 1px solid var(--border);
        }

        .sidebar-title {
            font-size: 1.25rem;
            font-weight: 400;
            margin-bottom: 0.25rem;
            display: flex;
            align-items: center;
            gap: 0.5rem;
            color: var(--text);
        }

        .sidebar-version {
            font-size: 0.875rem;
            color: var(--text-muted);
            margin-bottom: 0.75rem;
        }

        .sidebar-download {
            font-size: 0.875rem;
            color: var(--primary);
            text-decoration: none;
            font-weight: 500;
        }

        .sidebar-download:hover {
            text-decoration: underline;
        }

        .sidebar-nav {
            flex: 1;
            overflow-y: auto;
            padding: 0.5rem 0;
        }

        /* Nav Item */
        .nav-item {
            border-bottom: none;
        }

        .nav-item-header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 0.75rem 1.5rem;
            cursor: pointer;
            color: var(--sidebar-text);
            font-weight: 500;
            font-size: 0.875rem;
            transition: all 0.15s;
            user-select: none;
            border-radius: 0 24px 24px 0;
            margin-right: 0.5rem;
        }

        .nav-item-header:hover {
            background: var(--sidebar-hover);
            color: var(--text);
        }

        .nav-item-header.active {
            background: var(--sidebar-active);
            color: var(--primary);
            font-weight: 600;
        }

        .nav-item-header-content {
            display: flex;
            align-items: center;
            gap: 0.75rem;
        }

        .nav-item.has-children .nav-item-header::after {
            content: '';
            width: 6px;
            height: 6px;
            border-right: 2px solid currentColor;
            border-bottom: 2px solid currentColor;
            transform: rotate(-45deg);
            transition: transform 0.2s;
            margin-right: 0.5rem;
        }

        .nav-item.open .nav-item-header::after {
            transform: rotate(45deg);
        }

        /* Nav Children */
        .nav-children {
            max-height: 0;
            overflow: hidden;
            transition: max-height 0.3s ease;
        }

        .nav-item.open .nav-children {
            max-height: 2000px;
        }

        .nav-child-item {
            display: flex;
            align-items: center;
            padding: 0.5rem 1.5rem 0.5rem 3rem;
            color: var(--sidebar-text);
            font-size: 0.8125rem;
            text-decoration: none;
            cursor: pointer;
            transition: all 0.15s;
            border-radius: 0 24px 24px 0;
            margin-right: 0.5rem;
        }

        .nav-child-item:hover {
            background: var(--sidebar-hover);
            color: var(--text);
        }

        .nav-child-item.active {
            background: var(--sidebar-active);
            color: var(--primary);
            font-weight: 500;
        }

        .nav-group-label {
            padding: 0.5rem 1.5rem 0.25rem;
            font-size: 0.6875rem;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            color: var(--text-muted);
            margin-top: 0.5rem;
        }

        /* Method badge in sidebar */
        .nav-method {
            display: inline-block;
            width: 3rem;
            padding: 0.125rem 0;
            border-radius: 4px;
            font-size: 0.625rem;
            font-weight: 600;
            text-transform: uppercase;
            text-align: center;
            margin-right: 0.5rem;
            letter-spacing: 0.025em;
            flex-shrink: 0;
        }

        .nav-method.get { background: #e8f0fe; color: #1a73e8; }
        .nav-method.post { background: #e6f4ea; color: #137333; }
        .nav-method.patch { background: #fef3e8; color: #b06000; }
        .nav-method.delete { background: #fce8e8; color: #c5221f; }

        /* Main Content */
        .main-content {
            margin-left: var(--sidebar-width);
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            background: var(--bg);
        }

        .content-panels {
            flex: 1;
            padding: 2rem;
            max-width: 960px;
        }

        .content-panel {
            display: none;
        }

        .content-panel.active {
            display: block;
        }

        /* Content Header */
        .content-header {
            margin-bottom: 2rem;
            padding-bottom: 1.5rem;
            border-bottom: 1px solid var(--border);
        }

        .content-header h1 {
            font-size: 1.75rem;
            font-weight: 400;
            margin-bottom: 0.5rem;
            color: var(--text);
            letter-spacing: -0.25px;
        }

        .content-header p {
            color: var(--text-secondary);
            font-size: 1rem;
            line-height: 1.5;
        }

        /* Breadcrumb */
        .breadcrumb {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            font-size: 0.85rem;
            color: var(--text-muted);
            margin-bottom: 1rem;
        }

        .breadcrumb span:not(:last-child)::after {
            content: '/';
            margin-left: 0.5rem;
            color: var(--border);
        }

        /* Section Title */
        .section-title {
            font-size: 1.25rem;
            font-weight: 500;
            margin-bottom: 1rem;
            color: var(--text);
            letter-spacing: -0.25px;
        }

        /* Cards - Google style */
        .cards {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
            gap: 1.5rem;
            margin: 1.5rem 0;
        }

        .card {
            background: var(--card-bg);
            padding: 1.5rem;
            border-radius: 8px;
            border: 1px solid var(--border);
            transition: box-shadow 0.2s, border-color 0.2s;
            box-shadow: var(--card-shadow);
            cursor: pointer;
        }

        .card:hover {
            border-color: var(--primary);
            box-shadow: var(--card-shadow-hover);
        }

        .card h4 {
            color: var(--text);
            margin-bottom: 0.5rem;
            font-size: 1rem;
            font-weight: 500;
        }

        .card p {
            color: var(--text-secondary);
            font-size: 0.875rem;
            margin: 0;
            line-height: 1.5;
        }

        /* Code blocks - Google style */
        pre {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 8px;
            padding: 1rem;
            overflow-x: auto;
            font-family: 'Google Sans Mono', 'Fira Code', 'Roboto Mono', monospace;
            font-size: 0.8125rem;
            color: var(--text);
            margin: 1rem 0;
            line-height: 1.6;
        }

        code {
            font-family: 'Google Sans Mono', 'Fira Code', 'Roboto Mono', monospace;
            background: var(--bg-tertiary);
            padding: 0.125rem 0.375rem;
            border-radius: 4px;
            font-size: 0.8125rem;
            color: var(--google-red);
        }

        pre code {
            padding: 0;
            background: none;
            color: var(--text);
        }

        /* JSON Syntax Highlighting */
        .json-key { color: #7c3aed; }
        .json-string { color: #16a34a; }
        .json-number { color: #2563eb; }
        .json-boolean { color: #d97706; }
        .json-null { color: #9ca3af; font-style: italic; }

        /* Alert - Google style */
        .alert {
            padding: 1rem 1.25rem;
            border-radius: 8px;
            margin: 1rem 0;
            font-size: 0.875rem;
            line-height: 1.5;
        }

        .alert-info {
            background: #e8f0fe;
            border: 1px solid #d2e3fc;
            color: #1967d2;
        }

        /* Flow Steps - Google style */
        .flow-steps {
            margin: 1.5rem 0;
        }

        .flow-step {
            display: flex;
            gap: 1rem;
            margin-bottom: 0.75rem;
            padding: 1rem 1.25rem;
            background: var(--card-bg);
            border-radius: 8px;
            border: 1px solid var(--border);
            transition: box-shadow 0.2s;
            cursor: pointer;
        }

        .flow-step:hover {
            box-shadow: var(--card-shadow);
        }

        .step-number {
            width: 28px;
            height: 28px;
            display: flex;
            align-items: center;
            justify-content: center;
            background: var(--primary);
            color: white;
            font-weight: 500;
            font-size: 0.875rem;
            border-radius: 50%;
            flex-shrink: 0;
        }

        .step-content {
            flex: 1;
            color: var(--text);
            font-size: 0.9375rem;
            line-height: 1.6;
        }

        .step-title {
            font-weight: 500;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }

        .step-arrow {
            font-size: 0.75rem;
            transition: transform 0.2s;
        }

        .flow-step.open .step-arrow {
            transform: rotate(180deg);
        }

        .step-detail {
            max-height: 0;
            overflow: hidden;
            opacity: 0;
            margin-top: 0;
            color: var(--text-secondary);
            font-size: 0.875rem;
            font-weight: 400;
            line-height: 1.6;
            transition: max-height 0.3s ease, opacity 0.2s ease, margin-top 0.2s ease;
        }

        .flow-step.open .step-detail {
            max-height: 200px;
            opacity: 1;
            margin-top: 0.5rem;
        }

        /* Flow Diagram */
        #flow-diagram-container {
            width: 100%;
            height: 500px;
            background: var(--bg);
            border-radius: 8px;
            border: 1px solid var(--border);
        }

        /* Screen styles */
        .screen-layout {
            display: flex;
            gap: 2rem;
            align-items: flex-start;
        }

        .screen-content {
            flex: 1;
            min-width: 0;
        }

        .screen-preview {
            flex-shrink: 0;
            max-width: 280px;
            position: sticky;
            top: 2rem;
        }

        .screen-image {
            width: 100%;
            border-radius: 8px;
            border: 1px solid var(--border);
            background: var(--bg-secondary);
            box-shadow: var(--card-shadow);
        }

        .screen-platforms {
            display: inline-flex;
            gap: 0.5rem;
            margin-bottom: 1rem;
        }

        .platform-badge {
            padding: 0.25rem 0.625rem;
            border-radius: 12px;
            font-size: 0.75rem;
            font-weight: 500;
            background: var(--primary-light);
            color: var(--primary);
        }

        .screen-call-row {
            cursor: pointer;
        }

        .screen-call-row:hover td {
            background: var(--primary-light);
        }

        /* Endpoint Detail */
        .endpoint-detail {
            margin-bottom: 2rem;
        }

        .endpoint-header {
            display: flex;
            align-items: center;
            gap: 1rem;
            margin-bottom: 1rem;
            flex-wrap: wrap;
        }

        .method {
            padding: 0.25rem 0.625rem;
            border-radius: 4px;
            font-weight: 600;
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.025em;
        }

        .method.get { background: #e8f0fe; color: #1a73e8; }
        .method.post { background: #e6f4ea; color: #137333; }
        .method.patch { background: #fef3e8; color: #b06000; }
        .method.delete { background: #fce8e8; color: #c5221f; }

        .endpoint-path {
            font-family: 'Google Sans Mono', 'Fira Code', monospace;
            color: var(--text);
            font-weight: 500;
            font-size: 1.125rem;
        }

        .endpoint-description {
            color: var(--text-secondary);
            margin-bottom: 1.5rem;
            line-height: 1.6;
            font-size: 0.9375rem;
        }

        .auth-badge {
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
            padding: 0.5rem 0.875rem;
            background: var(--primary-light);
            border: 1px solid #d2e3fc;
            border-radius: 16px;
            font-size: 0.8125rem;
            margin-bottom: 1.5rem;
            color: var(--primary);
            font-weight: 500;
        }

        /* Table - Google style */
        table {
            width: 100%;
            border-collapse: collapse;
            margin: 1rem 0;
            font-size: 0.875rem;
        }

        th, td {
            padding: 0.75rem 1rem;
            text-align: left;
            border-bottom: 1px solid var(--border);
        }

        th {
            color: var(--text);
            font-weight: 500;
            background: var(--bg-secondary);
            font-size: 0.8125rem;
            text-transform: uppercase;
            letter-spacing: 0.025em;
        }

        td {
            color: var(--text-secondary);
        }

        tr:hover td {
            background: var(--bg-secondary);
        }

        /* Badge - Google style */
        .badge {
            padding: 0.125rem 0.5rem;
            border-radius: 10px;
            font-size: 0.6875rem;
            font-weight: 500;
            text-transform: uppercase;
            letter-spacing: 0.025em;
        }

        .badge.required {
            background: #fce8e8;
            color: #c5221f;
        }

        .badge.optional {
            background: var(--bg-tertiary);
            color: var(--text-secondary);
        }

        /* Code Block with Header - Google style */
        .code-block {
            margin: 1.5rem 0;
            border: 1px solid var(--border);
            border-radius: 8px;
            overflow: hidden;
            box-shadow: var(--card-shadow);
        }

        .code-header {
            background: var(--bg-secondary);
            padding: 0.625rem 1rem;
            font-size: 0.75rem;
            font-weight: 500;
            color: var(--text-secondary);
            border-bottom: 1px solid var(--border);
            text-transform: uppercase;
            letter-spacing: 0.025em;
        }

        .code-block pre {
            margin: 0;
            border: none;
            border-radius: 0;
            background: var(--bg-secondary);
        }

        /* Try It Section */
        .try-it-section {
            margin-top: 2rem;
            padding-top: 1.5rem;
            border-top: 1px solid var(--border);
        }

        .try-it-btn {
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
            padding: 0.5rem 1.25rem;
            background: var(--primary);
            color: white;
            border: none;
            border-radius: 4px;
            font-weight: 500;
            font-size: 0.875rem;
            cursor: pointer;
            transition: background 0.2s, box-shadow 0.2s;
            box-shadow: 0 1px 2px 0 rgba(26,115,232,0.3);
        }

        .try-it-btn:hover {
            background: var(--primary-dark);
            box-shadow: 0 2px 4px 0 rgba(26,115,232,0.4);
        }

        /* Global API Tester */
        .global-tester {
            margin-top: 1rem;
            background: var(--card-bg);
            border: 1px solid var(--border);
            border-radius: 8px;
            padding: 1.5rem;
            box-shadow: var(--card-shadow);
        }

        .tester-config {
            margin-bottom: 1rem;
        }

        .tester-row {
            display: flex;
            gap: 0.75rem;
            margin-bottom: 1rem;
        }

        .method-select {
            padding: 0.5rem 0.75rem;
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 4px;
            color: var(--text);
            font-weight: 500;
            font-size: 0.875rem;
        }

        .url-input {
            flex: 1;
            padding: 0.5rem 0.75rem;
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 4px;
            color: var(--text);
            font-size: 0.875rem;
        }

        .base-url-select {
            padding: 0.5rem 0.5rem;
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 4px;
            color: var(--text);
            font-size: 0.8125rem;
            font-weight: 500;
            max-width: 140px;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }

        .base-url-selector {
            padding: 0.5rem 0.75rem;
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 4px;
            color: var(--text);
            font-size: 0.875rem;
            font-family: 'Google Sans Mono', monospace;
        }

        .send-btn {
            padding: 0.5rem 1.25rem;
            background: var(--primary);
            color: white;
            border: none;
            border-radius: 4px;
            font-weight: 500;
            font-size: 0.875rem;
            cursor: pointer;
            transition: background 0.2s, box-shadow 0.2s;
            box-shadow: 0 1px 2px 0 rgba(26,115,232,0.3);
        }

        .send-btn:hover {
            background: var(--primary-dark);
            box-shadow: 0 2px 4px 0 rgba(26,115,232,0.4);
        }

        .tester-tabs {
            display: flex;
            gap: 0;
            margin-bottom: 1rem;
            border-bottom: 1px solid var(--border);
        }

        .tab-btn {
            padding: 0.625rem 1rem;
            background: none;
            border: none;
            color: var(--text-secondary);
            cursor: pointer;
            border-bottom: 2px solid transparent;
            transition: all 0.15s;
            font-size: 0.875rem;
            font-weight: 500;
        }

        .tab-btn:hover {
            color: var(--text);
        }

        .tab-btn.active {
            color: var(--primary);
            border-bottom-color: var(--primary);
        }

        .tab-content {
            display: none;
        }

        .tab-content.active {
            display: block;
        }

        .param-row, .formdata-row {
            display: flex;
            gap: 0.75rem;
            margin-bottom: 0.75rem;
            align-items: center;
        }

        .param-key, .param-value, .formdata-key, .formdata-value {
            flex: 1;
            padding: 0.5rem 0.75rem;
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 4px;
            color: var(--text);
            font-size: 0.875rem;
        }

        .icon-btn {
            width: 36px;
            height: 36px;
            border-radius: 4px;
            border: none;
            background: var(--primary);
            color: white;
            font-size: 1.25rem;
            cursor: pointer;
            transition: background 0.2s, box-shadow 0.2s;
            box-shadow: 0 1px 2px 0 rgba(26,115,232,0.3);
        }

        .icon-btn:hover {
            background: var(--primary-dark);
            box-shadow: 0 2px 4px 0 rgba(26,115,232,0.4);
        }

        .icon-btn.danger {
            background: var(--danger);
            box-shadow: 0 1px 2px 0 rgba(234,67,53,0.3);
        }

        .icon-btn.danger:hover {
            background: #c5221f;
            box-shadow: 0 2px 4px 0 rgba(234,67,53,0.4);
        }

        textarea {
            width: 100%;
            min-height: 120px;
            padding: 0.75rem;
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 4px;
            color: var(--text);
            font-family: 'Google Sans Mono', 'Fira Code', monospace;
            font-size: 0.875rem;
            resize: vertical;
            line-height: 1.5;
        }

        .response-section {
            margin-top: 1rem;
            border: 1px solid var(--border);
            border-radius: 8px;
            overflow: hidden;
            box-shadow: var(--card-shadow);
        }

        .response-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 0.75rem 1rem;
            background: var(--bg-secondary);
            border-bottom: 1px solid var(--border);
            font-size: 0.875rem;
            font-weight: 500;
            color: var(--text-secondary);
        }

        .response-body {
            padding: 1rem;
            margin: 0;
            max-height: 300px;
            overflow: auto;
            background: var(--bg);
        }

        .status-badge {
            padding: 0.25rem 0.625rem;
            border-radius: 10px;
            font-size: 0.75rem;
            font-weight: 500;
        }

        .status-success { background: #e6f4ea; color: #137333; }
        .status-error { background: #fce8e8; color: #c5221f; }

        /* Form Group - Google style */
        .form-group {
            margin-bottom: 1rem;
        }

        .form-group label {
            display: block;
            margin-bottom: 0.375rem;
            font-weight: 500;
            color: var(--text);
            font-size: 0.875rem;
        }

        .form-group input {
            width: 100%;
            padding: 0.625rem 0.875rem;
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 4px;
            color: var(--text);
            font-size: 0.875rem;
            transition: border-color 0.2s, box-shadow 0.2s;
        }

        .form-group input:focus {
            outline: none;
            border-color: var(--primary);
            box-shadow: 0 0 0 2px rgba(26,115,232,0.2);
        }

        .form-group small {
            display: block;
            margin-top: 0.375rem;
            color: var(--text-secondary);
            font-size: 0.75rem;
        }

        /* Auth Selector - Google style */
        .auth-selector {
            display: flex;
            background: var(--bg-secondary);
            border-radius: 20px;
            padding: 4px;
            gap: 4px;
            margin-bottom: 1rem;
            border: 1px solid var(--border);
        }

        .auth-option {
            flex: 1;
            position: relative;
        }

        .auth-option input {
            position: absolute;
            opacity: 0;
        }

        .auth-option label {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 0.5rem;
            padding: 0.5rem 1rem;
            border-radius: 16px;
            font-weight: 500;
            font-size: 0.8125rem;
            cursor: pointer;
            transition: all 0.15s;
            color: var(--text-secondary);
        }

        .auth-option input:checked + label {
            background: var(--primary);
            color: white;
            box-shadow: 0 1px 2px 0 rgba(26,115,232,0.3);
        }

        /* Credentials Panel */
        .credentials-panel {
            padding: 1rem;
            margin: 0 0.5rem;
        }

        .cred-row {
            margin-bottom: 1rem;
        }

        .cred-row label {
            display: block;
            font-size: 0.75rem;
            font-weight: 500;
            color: var(--text-secondary);
            margin-bottom: 0.25rem;
            text-transform: uppercase;
            letter-spacing: 0.025em;
        }

        .cred-input-wrapper {
            position: relative;
        }

        .cred-row input {
            width: 100%;
            padding: 0.5rem 2rem 0.5rem 0.75rem;
            border: 1px solid var(--border);
            border-radius: 4px;
            font-size: 0.875rem;
            font-family: 'Google Sans Mono', monospace;
        }

        .cred-row input:focus {
            outline: none;
            border-color: var(--primary);
            box-shadow: 0 0 0 2px rgba(26,115,232,0.2);
        }

        .cred-toggle {
            position: absolute;
            right: 0.5rem;
            top: 50%;
            transform: translateY(-50%);
            background: none;
            border: none;
            cursor: pointer;
            color: var(--text-muted);
            padding: 0.25rem;
            font-size: 0.875rem;
        }

        .cred-toggle:hover {
            color: var(--text);
        }

        .cred-row small {
            display: block;
            margin-top: 0.25rem;
            color: var(--text-muted);
            font-size: 0.75rem;
        }

        /* Footer - Google style */
        .footer {
            background: var(--bg-secondary);
            border-top: 1px solid var(--border);
            padding: 1.5rem 2rem;
            text-align: center;
            color: var(--text-secondary);
            font-size: 0.8125rem;
        }

        .footer a {
            color: var(--primary);
            text-decoration: none;
            font-weight: 500;
        }

        .footer a:hover {
            text-decoration: underline;
        }

        /* Responsive */
        @media (max-width: 768px) {
            .mobile-header {
                display: flex;
            }

            .sidebar {
                transform: translateX(-100%);
                transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
                box-shadow: 2px 0 8px rgba(0,0,0,0.15);
            }

            .sidebar.mobile-open {
                transform: translateX(0);
            }

            .sidebar-overlay {
                display: block;
            }

            .main-content {
                margin-left: 0;
                margin-top: 64px;
            }

            .content-panels {
                padding: 1rem;
            }

            #flow-diagram-container {
                height: 350px;
            }

            .tester-row {
                flex-direction: column;
            }

            .screen-layout {
                flex-direction: column;
            }

            .screen-preview {
                max-width: 100%;
            }
        }
    </style>
</head>
<body>
    <!-- Mobile Header -->
    <div class="mobile-header">
        <button class="hamburger" onclick="toggleSidebar()">
            <span></span>
            <span></span>
            <span></span>
        </button>
        <span class="mobile-title">{{.Info.Title}}</span>
    </div>

    <!-- Sidebar Overlay -->
    <div class="sidebar-overlay" onclick="toggleSidebar()"></div>

    <!-- Sidebar -->
    <aside class="sidebar">
        <div class="sidebar-header">
            <div class="sidebar-title">🏛️ {{.Info.Title}}</div>
            <div class="sidebar-version">v{{.Info.Version}}</div>
            <a href="/api/docs/yaml" class="sidebar-download" download>📥 Download YAML</a>
        </div>

        <nav class="sidebar-nav">
            <!-- Overview -->
            <div class="nav-item">
                <div class="nav-item-header" data-target="panel-overview">
                    <span class="nav-item-header-content">📋 Overview</span>
                </div>
            </div>

            <!-- Example Flow -->
            <div class="nav-item has-children">
                <div class="nav-item-header" onclick="toggleCollapse(this)">
                    <span class="nav-item-header-content">🔄 Example Flow</span>
                </div>
                <div class="nav-children">
                    {{range $fi, $flow := .FlowOverview.Methods}}
                    <a class="nav-child-item" data-target="panel-flow-{{$fi}}">{{$flow.Type}} Flow</a>
                    {{end}}
                    <a class="nav-child-item" data-target="panel-flow-diagram">Service Flow Diagram</a>
                </div>
            </div>

            <!-- Endpoint List -->
            <div class="nav-item has-children">
                <div class="nav-item-header" onclick="toggleCollapse(this)">
                    <span class="nav-item-header-content">🔌 Endpoints</span>
                </div>
                <div class="nav-children">
                    {{range $si, $section := .Sections}}
                    {{if .Endpoints}}
                    <div class="nav-group-label">{{.Title}}</div>
                    {{range $ei, $ep := .Endpoints}}
                    <a class="nav-child-item" data-target="panel-endpoint-{{$si}}-{{$ei}}">
                        <span class="nav-method {{lower .Method}}">{{.Method}}</span>
                        <span>{{.Path}}</span>
                    </a>
                    {{end}}
                    {{end}}
                    {{end}}
                </div>
            </div>

            <!-- Guides -->
            {{range $gi, $guide := .Guides}}
            <div class="nav-item">
                <div class="nav-item-header" data-target="panel-guide-{{$gi}}">
                    <span class="nav-item-header-content">{{$guide.Icon}} {{$guide.Title}}</span>
                </div>
            </div>
            {{end}}

            <!-- Screens -->
            {{if .Screens}}
            <div class="nav-item has-children">
                <div class="nav-item-header" onclick="toggleCollapse(this)">
                    <span class="nav-item-header-content">📱 Screens</span>
                </div>
                <div class="nav-children">
                    {{range $si, $screen := .Screens}}
                    <a class="nav-child-item" data-target="panel-screen-{{$si}}">
                        {{$screen.Icon}} {{$screen.Title}}
                    </a>
                    {{end}}
                </div>
            </div>
            {{end}}

            <!-- Credentials -->
            <div class="nav-item">
                <div class="nav-item-header" data-target="panel-credentials">
                    <span class="nav-item-header-content">🔐 Credentials</span>
                </div>
            </div>
        </nav>
    </aside>

    <!-- Main Content -->
    <main class="main-content">
        <div class="content-panels">
            <!-- Overview Panel -->
            <div class="content-panel active" id="panel-overview">
                <div class="content-header">
                    <h1>📋 Overview</h1>
                    <p>{{.Info.Description}}</p>
                </div>

                <div class="cards">
                    {{range $ci, $card := .Info.OverviewCards}}
                    <div class="card" onclick="{{if $card.Content}}showPanel('panel-card-{{$ci}}'){{end}}">
                        <h4>{{$card.Icon}} {{$card.Title}}</h4>
                        <p>{{$card.Description}}</p>
                    </div>
                    {{end}}
                </div>

                <h3 class="section-title">Base URL</h3>
                {{if .Info.BaseURLs}}
                <select class="base-url-selector" id="global-base-url" onchange="updateGlobalBaseURL()">
                    {{range .Info.BaseURLs}}
                    <option value="{{.URL}}" {{if .Default}}selected{{end}}>{{.Label}} ({{.URL}})</option>
                    {{end}}
                </select>
                {{else}}
                <pre><code>{{.Info.BaseURL}}</code></pre>
                {{end}}

                <h3 class="section-title">Constraints</h3>
                <ul>
                    {{range .Constraints}}
                    <li>{{.}}</li>
                    {{end}}
                </ul>
            </div>

            <!-- Overview Card Content Panels -->
            {{range $ci, $card := .Info.OverviewCards}}
            {{if $card.Content}}
            <div class="content-panel" id="panel-card-{{$ci}}">
                <div class="content-header">
                    <h1>{{$card.Icon}} {{$card.Title}}</h1>
                </div>
                <div style="line-height:1.8; color:var(--text);">{{$card.Content | md}}</div>
                <div style="margin-top:2rem;">
                    <button class="try-it-btn" onclick="showPanel('panel-overview')">← Back to Overview</button>
                </div>
            </div>
            {{end}}
            {{end}}

            <!-- Dynamic Auth Flow Panels -->
            {{range $fi, $flow := .FlowOverview.Methods}}
            <div class="content-panel" id="panel-flow-{{$fi}}">
                <div class="content-header">
                    <h1>🔄 {{$flow.Type}} Authentication Flow</h1>
                    <p>Step-by-step authentication menggunakan {{$flow.Type}}</p>
                </div>

                <div class="flow-steps">
                    {{range $i, $step := $flow.Steps}}
                    <div class="flow-step" onclick="toggleStepDetail(this)">
                        <div class="step-number">{{add $i 1}}</div>
                        <div class="step-content">
                            <div class="step-title">{{$step.Title}} <span class="step-arrow">▾</span></div>
                            {{if $step.Detail}}
                            <div class="step-detail">{{$step.Detail}}</div>
                            {{end}}
                        </div>
                    </div>
                    {{end}}
                </div>

                {{if $.FlowOverview.Note}}
                <div class="alert alert-info">{{$.FlowOverview.Note}}</div>
                {{end}}
            </div>
            {{end}}

            <!-- Flow Diagram Panel -->
            <div class="content-panel" id="panel-flow-diagram">
                <div class="content-header">
                    <h1>🔄 Service Flow Diagram</h1>
                    <p>Visualisasi alur data antara services</p>
                </div>
                <div id="flow-diagram-container"></div>
            </div>

            <!-- Endpoint Panels -->
            {{range $si, $section := .Sections}}
            {{if .Endpoints}}
            {{range $ei, $ep := .Endpoints}}
            <div class="content-panel" id="panel-endpoint-{{$si}}-{{$ei}}">
                <div class="breadcrumb">
                    <span>{{$section.Title}}</span>
                    <span>{{$ep.Name}}</span>
                </div>

                <div class="endpoint-detail">
                    <div class="endpoint-header">
                        <span class="method {{lower $ep.Method}}">{{$ep.Method}}</span>
                        <span class="endpoint-path">{{$ep.Path}}</span>
                    </div>

                    <p class="endpoint-description">{{$ep.Description}}</p>

                    {{if eq $ep.Auth "required"}}
                    <div class="auth-badge">
                        🔒 Auth Required {{if $ep.Permission}}<code>{{$ep.Permission}}</code>{{end}}
                    </div>
                    {{else if eq $ep.Auth "optional"}}
                    <div class="auth-badge">🔓 Optional Auth (Public available)</div>
                    {{end}}

                    {{if $ep.QueryParams}}
                    <h3 class="section-title">Query Parameters</h3>
                    <table>
                        <tr><th>Param</th><th>Type</th><th>Required</th><th>Description</th></tr>
                        {{range $ep.QueryParams}}
                        <tr>
                            <td>{{.Name}}</td>
                            <td>{{.Type}}</td>
                            <td><span class="badge {{if .Required}}required{{else}}optional{{end}}">{{if .Required}}required{{else}}optional{{end}}</span></td>
                            <td>{{.Description}}{{if .Default}} (default: {{.Default}}){{end}}</td>
                        </tr>
                        {{end}}
                    </table>
                    {{end}}

                    {{if $ep.Body}}
                    <h3 class="section-title">Request Body Fields</h3>
                    <table>
                        <tr><th>Field</th><th>Type</th><th>Required</th><th>Description</th></tr>
                        {{range $ep.Body}}
                        <tr>
                            <td>{{.Name}}</td>
                            <td>{{.Type}}</td>
                            <td><span class="badge {{if .Required}}required{{else}}optional{{end}}">{{if .Required}}required{{else}}optional{{end}}</span></td>
                            <td>{{.Description}}{{if .Example}} <code>ex: {{.Example}}</code>{{end}}</td>
                        </tr>
                        {{end}}
                    </table>
                    {{end}}

                    {{if $ep.ExampleBody}}
                    <div class="code-block">
                        <div class="code-header">Example Request Body</div>
                        <pre class="json-highlight"><code>{{$ep.ExampleBody}}</code></pre>
                    </div>
                    {{end}}

                    {{if $ep.ExampleResponse}}
                    <div class="code-block">
                        <div class="code-header">Example Response</div>
                        <pre class="json-highlight"><code>{{$ep.ExampleResponse}}</code></pre>
                    </div>
                    {{end}}

                    <!-- Try It Section -->
                    <div class="try-it-section">
                        <button class="try-it-btn" onclick="openTester({{$si}}, {{$ei}})">
                            ▶ Try It
                        </button>
                        <div class="global-tester" id="tester-endpoint-{{$si}}-{{$ei}}" style="display:none">
                            <div class="tester-config">
                                <div class="form-group">
                                    <label>Authentication Method</label>
                                    <div class="auth-selector">
                                        {{range $ai, $mode := $.APITesterDefaults.AuthModes}}
                                        <div class="auth-option">
                                            <input type="radio" name="auth-method-{{$si}}-{{$ei}}" value="{{$mode.Name}}" id="auth-{{$si}}-{{$ei}}-{{$ai}}" {{if eq $ai 0}}checked{{end}} onchange="updateAuthDisplay({{$si}}, {{$ei}})">
                                            <label for="auth-{{$si}}-{{$ei}}-{{$ai}}">{{$mode.Name}}</label>
                                        </div>
                                        {{end}}
                                        <div class="auth-option">
                                            <input type="radio" name="auth-method-{{$si}}-{{$ei}}" value="none" id="auth-none-{{$si}}-{{$ei}}" onchange="updateAuthDisplay({{$si}}, {{$ei}})">
                                            <label for="auth-none-{{$si}}-{{$ei}}">Public</label>
                                        </div>
                                    </div>
                                </div>
                                <!-- Display only, not input -->
                                <div class="form-group" id="auth-display-{{$si}}-{{$ei}}" style="display:none">
                                    <label>Using <span class="auth-type-label"></span> from Credentials</label>
                                    <input type="text" class="inline-auth-preview" readonly style="background:var(--bg-secondary); color:var(--text-secondary);">
                                </div>
                            </div>
                            <div class="tester-row">
                                <select class="method-select inline-method" data-section="{{$si}}" data-endpoint="{{$ei}}">
                                    {{range $mi, $m := $.APITesterDefaults.Methods}}<option value="{{$m}}"{{if eq $ep.Method $m}} selected{{end}}>{{$m}}</option>
                                    {{end}}
                                </select>
                                {{if $.Info.BaseURLs}}
                                <select class="base-url-select inline-base-url" data-section="{{$si}}" data-endpoint="{{$ei}}" onchange="updateEndpointBaseURL({{$si}}, {{$ei}})">
                                    {{range $bi, $bu := $.Info.BaseURLs}}<option value="{{$bu.URL}}" {{if $bu.Default}}selected{{end}}>{{$bu.Label}}</option>
                                    {{end}}
                                </select>
                                {{end}}
                                <input type="text" class="url-input inline-url" data-section="{{$si}}" data-endpoint="{{$ei}}" value="{{$.Info.BaseURL}}{{$ep.Path}}" data-path="{{$ep.Path}}">
                                <button class="send-btn" onclick="sendInlineRequest({{$si}}, {{$ei}})">Send</button>
                            </div>
                            <div class="tester-tabs">
                                <button class="tab-btn active" onclick="switchInlineTab({{$si}}, {{$ei}}, 'body')">Body</button>
                                <button class="tab-btn" onclick="switchInlineTab({{$si}}, {{$ei}}, 'headers')">Headers</button>
                            </div>
                            <div class="tab-content active" id="tab-body-{{$si}}-{{$ei}}">
                                <textarea class="inline-body" data-section="{{$si}}" data-endpoint="{{$ei}}">{{$ep.ExampleBody}}</textarea>
                            </div>
                            <div class="tab-content" id="tab-headers-{{$si}}-{{$ei}}">
                                <div class="header-row" style="display:flex; gap:0.75rem; margin-bottom:0.5rem;">
                                    <input type="text" value="Content-Type" class="header-key" readonly style="flex:1; padding:0.5rem; background:var(--bg); border:1px solid var(--border); border-radius:6px; color:var(--text);">
                                    <input type="text" value="application/json" class="header-value" style="flex:2; padding:0.5rem; background:var(--bg); border:1px solid var(--border); border-radius:6px; color:var(--text);">
                                </div>
                            </div>
                            <div class="response-section">
                                <div class="response-header">
                                    <span>Response</span>
                                    <span class="status-badge inline-status" data-section="{{$si}}" data-endpoint="{{$ei}}"></span>
                                </div>
                                <pre class="response-body inline-response" data-section="{{$si}}" data-endpoint="{{$ei}}"><code>Click "Send" to see response...</code></pre>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
            {{end}}
            {{end}}
            {{end}}

            <!-- Guide Panels -->
            {{range $gi, $guide := .Guides}}
            <div class="content-panel" id="panel-guide-{{$gi}}">
                <div class="content-header">
                    <h1>{{$guide.Icon}} {{$guide.Title}}</h1>
                    <p>{{$guide.Description}}</p>
                </div>

                {{range .Flow}}
                <h3 class="section-title">Step {{.Step}}: {{.Title}}</h3>
                {{if .Description}}
                <p style="color:var(--text-secondary); margin-bottom:1rem; line-height:1.6;">{{.Description}}</p>
                {{end}}
                {{if .Endpoint}}
                <div class="endpoint-detail">
                    <div class="endpoint-header">
                        <span class="method {{lower .Endpoint.Method}}">{{.Endpoint.Method}}</span>
                        <span class="endpoint-path">{{.Endpoint.Path}}</span>
                    </div>
                    <p style="color:var(--text-secondary); margin-bottom:1rem;">Service: {{.Endpoint.Service}}</p>
                    {{if .Endpoint.ContentType}}
                    <p style="color:var(--text-secondary); margin-bottom:1rem;">Content-Type: <code>{{.Endpoint.ContentType}}</code></p>
                    {{end}}
                    {{if .Endpoint.Fields}}
                    <h3 class="section-title">Fields</h3>
                    <table>
                        <tr><th>Field</th><th>Type</th><th>Required</th><th>Description</th></tr>
                        {{range .Endpoint.Fields}}
                        <tr>
                            <td>{{.Name}}</td>
                            <td>{{.Type}}</td>
                            <td><span class="badge {{if .Required}}required{{else}}optional{{end}}">{{if .Required}}required{{else}}optional{{end}}</span></td>
                            <td>{{.Description}}</td>
                        </tr>
                        {{end}}
                    </table>
                    {{end}}
                    {{if .CurlExampleJWT}}
                    <div class="code-block">
                        <div class="code-header">cURL Example (JWT Bearer)</div>
                        <pre><code>{{.CurlExampleJWT}}</code></pre>
                    </div>
                    {{end}}
                    {{if .CurlExampleAPIKey}}
                    <div class="code-block">
                        <div class="code-header">cURL Example (API Key)</div>
                        <pre><code>{{.CurlExampleAPIKey}}</code></pre>
                    </div>
                    {{end}}
                    {{if .CurlExample}}
                    <div class="code-block">
                        <div class="code-header">cURL Example</div>
                        <pre><code>{{.CurlExample}}</code></pre>
                    </div>
                    {{end}}
                    {{if .ResponseExample}}
                    <div class="code-block">
                        <div class="code-header">Response Example</div>
                        <pre class="json-highlight"><code>{{.ResponseExample}}</code></pre>
                    </div>
                    {{end}}
                </div>
                {{end}}
                {{if .Actions}}
                <h3 class="section-title">Next Steps</h3>
                {{range .Actions}}
                <div class="flow-step" style="cursor:default;">
                    <div class="step-content">
                        <strong>{{.Description}}</strong>
                        <p style="color:var(--text-secondary); margin-top:0.25rem;">Endpoint: <code>{{.Endpoint}}</code></p>
                    </div>
                </div>
                {{end}}
                {{end}}
                {{end}}
            </div>
            {{end}}

            <!-- Screen Panels -->
            {{range $si, $screen := .Screens}}
            <div class="content-panel" id="panel-screen-{{$si}}">
                <div class="content-header">
                    <h1>{{$screen.Icon}} {{$screen.Title}}</h1>
                    <p>{{$screen.Description}}</p>
                </div>

                <div class="screen-layout">
                    <div class="screen-content">
                        {{if $screen.Platform}}
                        <div class="screen-platforms">
                            {{range $screen.Platform}}
                            <span class="platform-badge">{{.}}</span>
                            {{end}}
                        </div>
                        {{end}}

                        <h3 class="section-title">API Calls</h3>
                        <table>
                            <tr><th>Method</th><th>Endpoint</th><th>Purpose</th><th>Trigger</th><th>Auth</th></tr>
                            {{range $screen.Calls}}
                            <tr class="screen-call-row">
                                <td><span class="method {{lower .Method}}">{{.Method}}</span></td>
                                <td><code>{{.Path}}</code></td>
                                <td>{{.Purpose}}</td>
                                <td style="color:var(--text-muted); font-size:0.8125rem;">{{.Trigger}}</td>
                                <td>{{if eq .Auth "required"}}<span class="badge required">required</span>{{else if eq .Auth "optional"}}<span class="badge optional">optional</span>{{else}}<span class="badge optional">none</span>{{end}}</td>
                            </tr>
                            {{if .Notes}}
                            <tr class="screen-call-row">
                                <td colspan="5" style="padding:0 1rem 0.75rem 1rem; font-size:0.8125rem; color:var(--text-muted); border-bottom: 1px solid var(--border);">
                                    💡 {{.Notes}}
                                </td>
                            </tr>
                            {{end}}
                            {{end}}
                        </table>
                    </div>
                    {{if $screen.Image}}
                    <div class="screen-preview">
                        <img class="screen-image" src="{{$screen.Image}}" alt="{{$screen.Title}}" loading="lazy">
                    </div>
                    {{end}}
                </div>
            </div>
            {{end}}

            <!-- Credentials Panel -->
            <div class="content-panel" id="panel-credentials">
                <div class="content-header">
                    <h1>🔐 Credentials</h1>
                    <p>Konfigurasi credentials untuk testing API</p>
                </div>

                {{if .Info.BaseURLs}}
                <div style="margin-bottom:1.5rem;">
                    <label style="display:block; font-size:0.875rem; font-weight:500; color:var(--text); margin-bottom:0.5rem;">Environment</label>
                    <select class="base-url-selector" id="cred-env-select" onchange="switchCredEnvironment()">
                        {{range $bi, $bu := .Info.BaseURLs}}<option value="{{$bu.URL}}" {{if $bu.Default}}selected{{end}}>{{$bu.Label}} ({{$bu.URL}})</option>
                        {{end}}
                    </select>
                </div>
                {{end}}

                <div class="endpoint-detail" style="background:var(--card-bg); padding:2rem; border-radius:12px; border:1px solid var(--border); box-shadow:var(--card-shadow);">
                    {{range $ai, $mode := .APITesterDefaults.AuthModes}}
                    <div class="cred-row" style="margin-bottom:1.5rem;">
                        <label style="display:block; font-size:0.875rem; font-weight:500; color:var(--text); margin-bottom:0.5rem;">{{$mode.Name}}</label>
                        <div class="cred-input-wrapper" style="position:relative;">
                            <input type="password" id="cred-{{$ai}}" placeholder="{{$mode.Placeholder}}" onchange="saveCurrentCredential('{{$mode.Name}}', this.value)" style="width:100%; padding:0.75rem 3rem 0.75rem 1rem; border:1px solid var(--border); border-radius:8px; font-size:0.9375rem; font-family:'Google Sans Mono', monospace; background:var(--bg);">
                            <button class="cred-toggle" onclick="toggleVisibility('cred-{{$ai}}')" title="Show/Hide" style="position:absolute; right:0.75rem; top:50%; transform:translateY(-50%); background:none; border:none; cursor:pointer; color:var(--text-muted); padding:0.5rem; font-size:1rem;">👁</button>
                        </div>
                        <small style="display:block; margin-top:0.5rem; color:var(--text-muted); font-size:0.8125rem;">{{$mode.Header}}: {{$mode.Prefix}}&lt;value&gt;</small>
                    </div>
                    {{end}}

                    <div class="alert alert-info" style="margin-top:1.5rem;">
                        <strong>💡 Info:</strong> Credentials tersimpan per environment di browser Anda (localStorage) dan tidak dikirim ke server.
                    </div>
                </div>

                <h3 class="section-title" style="margin-top:2rem;">Cara Menggunakan</h3>
                <div class="flow-steps">
                    <div class="flow-step">
                        <div class="step-number">1</div>
                        <div class="step-content">Isi credentials di atas</div>
                    </div>
                    <div class="flow-step">
                        <div class="step-number">2</div>
                        <div class="step-content">Klik "Endpoints" di sidebar untuk memilih API</div>
                    </div>
                    <div class="flow-step">
                        <div class="step-number">3</div>
                        <div class="step-content">Klik "Try It" pada endpoint yang ingin di-test</div>
                    </div>
                    <div class="flow-step">
                        <div class="step-number">4</div>
                        <div class="step-content">Pilih metode autentikasi dan klik Send</div>
                    </div>
                </div>
            </div>

            <!-- Authentication Panel -->
            <div class="content-panel" id="panel-authentication">
                <div class="content-header">
                    <h1>🔐 Authentication</h1>
                    <p>Methods and permissions for API access</p>
                </div>

                {{if .Authentication.Methods}}
                <h3 class="section-title">Authentication Methods</h3>
                {{range .Authentication.Methods}}
                <div class="endpoint-detail" style="background:var(--card-bg); padding:1.5rem; border-radius:8px; margin-bottom:1rem;">
                    <h4 style="color:var(--primary); margin-bottom:1rem;">{{.Type}}</h4>
                    <p><strong>Header:</strong> <code>{{.Header}}: {{.Format}}</code></p>
                    <p><strong>Source:</strong> {{.Source}}</p>
                    <p>{{.Description}}</p>
                    {{if .Note}}<p class="text-muted"><em>Note: {{.Note}}</em></p>{{end}}
                    {{if .TokenContains}}
                    <div class="alert alert-info">
                        <strong>Token contains:</strong> {{join .TokenContains ", "}}
                    </div>
                    {{end}}
                </div>
                {{end}}
                {{end}}

                <h3 class="section-title">Permissions</h3>
                <table>
                    <tr><th>Permission</th><th>Description</th></tr>
                    {{range .Permissions}}
                    <tr>
                        <td><code>{{.Name}}</code></td>
                        <td>{{.Description}}</td>
                    </tr>
                    {{end}}
                </table>
            </div>
        </div>

        <!-- Footer -->
        <footer class="footer">
            <p>© 2026 {{.Info.Title}}. All rights reserved.</p>
            <p style="margin-top: 0.5rem;">
                <a href="/swagger/index.html">Swagger UI</a> •
                <a href="/swagger/doc.json">OpenAPI JSON</a> •
                <a href="/api/docs/yaml" download>📥 YAML</a> •
                <a href="/api/docs/spec">🤖 JSON</a>
            </p>
        </footer>
    </main>

    <!-- ReactFlow Component -->
    <script type="text/babel">
        const ReactFlow = window.ReactFlow.ReactFlow || window.ReactFlow.default;

        const nodes = [
            {{range .FlowDiagramNodes}}
            {
                id: '{{.ID}}',
                data: { label: '{{.Label | js}}' },
                position: { x: {{.Position.X}}, y: {{.Position.Y}} },
                style: {
                    background: '{{.Color}}',
                    color: 'white',
                    border: '1px solid {{.Color}}',
                    padding: '10px',
                    borderRadius: '8px',
                    fontSize: '14px',
                    fontWeight: '500'
                }
            },
            {{end}}
        ];

        const edges = [
            {{range .FlowDiagramEdges}}
            {
                id: '{{.Source}}-{{.Target}}',
                source: '{{.Source}}',
                target: '{{.Target}}',
                label: '{{.Label | js}}',
                animated: {{.Animated}},
                style: { stroke: '{{.Color}}' }
            },
            {{end}}
        ];

        function FlowDiagram() {
            return React.createElement('div', { style: { width: '100%', height: '100%' } },
                React.createElement(ReactFlow, {
                    nodes: nodes,
                    edges: edges,
                    fitView: true,
                    nodesDraggable: true,
                    nodesConnectable: false
                })
            );
        }

        const root = ReactDOM.createRoot(document.getElementById('flow-diagram-container'));
        root.render(React.createElement(FlowDiagram));
    </script>

    <script>
        // Credentials Management - per environment
        const CREDENTIALS_KEY_PREFIX = 'api_credentials_v4_';
        const baseUrlsConfig = JSON.parse({{.Info.BaseURLs | json}});
        const authModesConfig = JSON.parse({{.APITesterDefaults.AuthModes | json}});

        function getActiveBaseURL() {
            const envSelect = document.getElementById('cred-env-select');
            if (envSelect) return envSelect.value;
            const globalSelect = document.getElementById('global-base-url');
            if (globalSelect) return globalSelect.value;
            return '';
        }

        function getCredKey() {
            return CREDENTIALS_KEY_PREFIX + getActiveBaseURL();
        }

        function loadCredentials() {
            const stored = localStorage.getItem(getCredKey());
            const creds = stored ? JSON.parse(stored) : {};
            authModesConfig.forEach((mode, idx) => {
                const input = document.getElementById('cred-' + idx);
                if (input) input.value = creds[mode.name] || '';
            });
        }

        function saveCurrentCredential(modeName, value) {
            const stored = localStorage.getItem(getCredKey());
            const creds = stored ? JSON.parse(stored) : {};
            creds[modeName] = value;
            localStorage.setItem(getCredKey(), JSON.stringify(creds));
        }

        function switchCredEnvironment() {
            loadCredentials();
        }

        function getCredentialsForBaseURL(baseUrl) {
            const stored = localStorage.getItem(CREDENTIALS_KEY_PREFIX + baseUrl);
            return stored ? JSON.parse(stored) : {};
        }

        // Toggle password visibility
        function toggleVisibility(inputId) {
            const input = document.getElementById(inputId);
            input.type = input.type === 'password' ? 'text' : 'password';
        }

        // Load saved tokens
        document.addEventListener('DOMContentLoaded', function() {
            loadCredentials();

            // Highlight all static JSON blocks
            document.querySelectorAll('pre.json-highlight code').forEach(el => {
                try {
                    var parsed = JSON.parse(el.textContent);
                    el.innerHTML = syntaxHighlightJSON(parsed);
                } catch(e) {}
            });

            // Setup panel switching for nav items
            document.querySelectorAll('.nav-item-header[data-target]').forEach(el => {
                el.addEventListener('click', function() {
                    const target = this.getAttribute('data-target');
                    showPanel(target);
                });
            });

            document.querySelectorAll('.nav-child-item[data-target]').forEach(el => {
                el.addEventListener('click', function() {
                    const target = this.getAttribute('data-target');
                    showPanel(target);
                });
            });
        });

        // Panel switching
        function showPanel(targetId) {
            // Hide all panels
            document.querySelectorAll('.content-panel').forEach(el => el.classList.remove('active'));
            // Show target
            document.getElementById(targetId).classList.add('active');

            // Update sidebar active state
            document.querySelectorAll('.nav-item-header').forEach(el => el.classList.remove('active'));
            document.querySelectorAll('.nav-child-item').forEach(el => el.classList.remove('active'));

            const activeNav = document.querySelector('[data-target="' + targetId + '"]');
            if (activeNav) activeNav.classList.add('active');

            // Close mobile sidebar
            if (window.innerWidth <= 768) {
                toggleSidebar();
            }
        }

        // Sidebar collapse toggle
        function toggleCollapse(el) {
            const navItem = el.closest('.nav-item');
            navItem.classList.toggle('open');
        }

        // Flow step detail toggle
        function toggleStepDetail(el) {
            el.classList.toggle('open');
        }

        // Mobile sidebar toggle
        function toggleSidebar() {
            document.querySelector('.sidebar').classList.toggle('mobile-open');
            document.querySelector('.sidebar-overlay').classList.toggle('visible');
        }

        // Open inline tester
        function openTester(sectionIdx, endpointIdx) {
            const testerId = 'tester-endpoint-' + sectionIdx + '-' + endpointIdx;
            const tester = document.getElementById(testerId);

            if (tester.style.display === 'none') {
                tester.style.display = 'block';
                updateAuthDisplay(sectionIdx, endpointIdx);
            } else {
                tester.style.display = 'none';
            }
        }

        // Update all endpoint URLs when global base URL changes
        function updateGlobalBaseURL() {
            const globalSelect = document.getElementById('global-base-url');
            if (!globalSelect) return;
            const newBase = globalSelect.value;
            document.querySelectorAll('.inline-base-url').forEach(sel => {
                sel.value = newBase;
                const si = sel.dataset.section;
                const ei = sel.dataset.endpoint;
                const urlInput = document.querySelector('.inline-url[data-section="'+si+'"][data-endpoint="'+ei+'"]');
                if (urlInput) {
                    urlInput.value = newBase + (urlInput.dataset.path || '');
                }
            });
        }

        // Update single endpoint URL when its base URL dropdown changes
        function updateEndpointBaseURL(sectionIdx, endpointIdx) {
            const tester = document.getElementById('tester-endpoint-' + sectionIdx + '-' + endpointIdx);
            const baseSelect = tester.querySelector('.inline-base-url');
            const urlInput = tester.querySelector('.inline-url');
            if (baseSelect && urlInput) {
                urlInput.value = baseSelect.value + (urlInput.dataset.path || '');
            }
        }

        // Update auth display di tester
        function updateAuthDisplay(sectionIdx, endpointIdx) {
            const tester = document.getElementById('tester-endpoint-' + sectionIdx + '-' + endpointIdx);
            const authMethod = tester.querySelector('input[name="auth-method-' + sectionIdx + '-' + endpointIdx + '"]:checked').value;
            const displayDiv = document.getElementById('auth-display-' + sectionIdx + '-' + endpointIdx);
            const labelSpan = tester.querySelector('.auth-type-label');
            const previewInput = tester.querySelector('.inline-auth-preview');

            if (authMethod === 'none') {
                displayDiv.style.display = 'none';
                return;
            }

            displayDiv.style.display = 'block';
            const modeConfig = authModesConfig.find(m => m.name === authMethod);

            // Get credentials for the active base URL of this tester
            const baseSelect = tester.querySelector('.inline-base-url');
            const activeBase = baseSelect ? baseSelect.value : getActiveBaseURL();
            const envCreds = getCredentialsForBaseURL(activeBase);
            const credValue = modeConfig ? (envCreds[modeConfig.name] || '') : '';

            labelSpan.textContent = modeConfig ? modeConfig.name : authMethod;
            previewInput.value = credValue ? maskToken(credValue) : '(not set - configure in Credentials panel)';
        }

        // Mask token untuk display
        function maskToken(token) {
            if (!token || token.length < 15) return token || '';
            return token.substring(0, 12) + '...' + token.substring(token.length - 4);
        }

        // JSON syntax highlighting
        function syntaxHighlightJSON(json) {
            if (typeof json !== 'string') json = JSON.stringify(json, null, 2);
            json = json.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
            return json.replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g, function(match) {
                var cls = 'json-number';
                if (/^"/.test(match)) {
                    if (/:$/.test(match)) {
                        cls = 'json-key';
                    } else {
                        cls = 'json-string';
                    }
                } else if (/true|false/.test(match)) {
                    cls = 'json-boolean';
                } else if (/null/.test(match)) {
                    cls = 'json-null';
                }
                return '<span class="' + cls + '">' + match + '</span>';
            });
        }

        // Toggle inline auth method (deprecated, keeping for compatibility)
        function toggleInlineAuthMethod(sectionIdx, endpointIdx) {
            updateAuthDisplay(sectionIdx, endpointIdx);
        }

        // Switch inline tab
        function switchInlineTab(sectionIdx, endpointIdx, tabName) {
            var container = document.getElementById('tester-endpoint-' + sectionIdx + '-' + endpointIdx);
            container.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
            container.querySelectorAll('.tab-btn').forEach(el => el.classList.remove('active'));

            document.getElementById('tab-' + tabName + '-' + sectionIdx + '-' + endpointIdx).classList.add('active');
            event.target.classList.add('active');
        }

        // Send inline request
        async function sendInlineRequest(sectionIdx, endpointIdx) {
            const tester = document.getElementById('tester-endpoint-' + sectionIdx + '-' + endpointIdx);
            const method = tester.querySelector('.inline-method').value;
            let url = tester.querySelector('.inline-url').value;
            const authMethod = tester.querySelector('input[name="auth-method-' + sectionIdx + '-' + endpointIdx + '"]:checked').value;
            const body = tester.querySelector('.inline-body').value;
            const statusEl = tester.querySelector('.inline-status');
            const responseEl = tester.querySelector('.inline-response');

            // Build headers from credentials for active base URL
            const headers = {};

            if (authMethod !== 'none') {
                const modeConfig = authModesConfig.find(m => m.name === authMethod);
                if (modeConfig) {
                    const baseSelect = tester.querySelector('.inline-base-url');
                    const activeBase = baseSelect ? baseSelect.value : getActiveBaseURL();
                    const envCreds = getCredentialsForBaseURL(activeBase);
                    const credValue = envCreds[modeConfig.name] || '';
                    if (credValue) {
                        headers[modeConfig.header] = modeConfig.prefix + credValue;
                    }
                }
            }

            const options = { method: method, headers: headers };

            if (method !== 'GET' && body) {
                headers['Content-Type'] = 'application/json';
                options.body = body;
            }

            responseEl.innerHTML = '<code>Sending...</code>';
            statusEl.className = 'status-badge inline-status';

            try {
                const response = await fetch(url, options);
                const contentType = response.headers.get('content-type');
                let data;

                if (contentType && contentType.includes('application/json')) {
                    data = await response.json();
                } else {
                    data = await response.text();
                }

                statusEl.textContent = response.status + ' ' + response.statusText;
                statusEl.className = 'status-badge inline-status ' + (response.ok ? 'status-success' : 'status-error');

                if (typeof data === 'object') {
                    responseEl.innerHTML = '<code>' + syntaxHighlightJSON(data) + '</code>';
                } else {
                    // Try to parse and highlight if it looks like JSON
                    try {
                        var parsed = JSON.parse(data);
                        responseEl.innerHTML = '<code>' + syntaxHighlightJSON(parsed) + '</code>';
                    } catch(e) {
                        responseEl.innerHTML = '<code>' + data + '</code>';
                    }
                }
            } catch (error) {
                statusEl.textContent = 'Error';
                statusEl.className = 'status-badge inline-status status-error';
                responseEl.innerHTML = '<code>Error: ' + error.message + '</code>';
            }
        }
    </script>
</body>
</html>
`