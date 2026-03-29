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
            transition: opacity 0.3s;
        }

        .sidebar-overlay.visible {
            opacity: 1;
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
            z-index: 200;
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
            display: block;
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
            padding: 0.125rem 0.375rem;
            border-radius: 4px;
            font-size: 0.625rem;
            font-weight: 600;
            text-transform: uppercase;
            margin-right: 0.5rem;
            letter-spacing: 0.025em;
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

        /* Flow Diagram */
        #flow-diagram-container {
            width: 100%;
            height: 500px;
            background: var(--bg);
            border-radius: 8px;
            border: 1px solid var(--border);
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

        /* Quick Tests in sidebar */
        .quick-test-nav-item {
            display: flex;
            align-items: center;
            padding: 0.5rem 1.5rem 0.5rem 2rem;
            color: var(--sidebar-text);
            font-size: 0.8125rem;
            cursor: pointer;
            transition: all 0.15s;
            border-radius: 0 24px 24px 0;
            margin-right: 0.5rem;
        }

        .quick-test-nav-item:hover {
            background: var(--sidebar-hover);
            color: var(--text);
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
                    <a class="nav-child-item" data-target="panel-flow-jwt">JWT Authentication</a>
                    <a class="nav-child-item" data-target="panel-flow-apikey">API Key Authentication</a>
                    <a class="nav-child-item" data-target="panel-flow-diagram">Service Flow Diagram</a>
                </div>
            </div>

            <!-- Endpoint List -->
            <div class="nav-item has-children">
                <div class="nav-item-header" onclick="toggleCollapse(this)">
                    <span class="nav-item-header-content">🔌 Endpoint List</span>
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

            <!-- File Upload -->
            <div class="nav-item">
                <div class="nav-item-header" data-target="panel-file-upload">
                    <span class="nav-item-header-content">📤 File Upload</span>
                </div>
            </div>

            <!-- Quick Tests -->
            <div class="nav-item has-children">
                <div class="nav-item-header" onclick="toggleCollapse(this)">
                    <span class="nav-item-header-content">🧪 Quick Tests</span>
                </div>
                <div class="nav-children">
                    {{range $group, $tests := groupTests .APITesterDefaults.QuickTests}}
                    <div class="nav-group-label">{{$group}}</div>
                    {{range $tests}}
                    <a class="quick-test-nav-item" onclick="loadQuickTest('{{.ID}}')">
                        <span class="nav-method {{lower .Method}}">{{.Method}}</span>
                        <span>{{.Label}}</span>
                    </a>
                    {{end}}
                    {{end}}
                </div>
            </div>

            <!-- Authentication -->
            <div class="nav-item">
                <div class="nav-item-header" data-target="panel-authentication">
                    <span class="nav-item-header-content">🔐 Authentication</span>
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
                    <div class="card">
                        <h4>🏛️ Museum Management</h4>
                        <p>CRUD operations untuk museum dengan single museum pattern</p>
                    </div>
                    <div class="card">
                        <h4>🏺 Artifact Management</h4>
                        <p>Kelola koleksi artifact dengan metadata lengkap</p>
                    </div>
                    <div class="card">
                        <h4>📸 Media Integration</h4>
                        <p>Integrasi dengan Media Service untuk upload file</p>
                    </div>
                    <div class="card">
                        <h4>🔐 JWT Authentication</h4>
                        <p>Autentikasi via Account Service dengan RS256</p>
                    </div>
                </div>

                <h3 class="section-title">Base URL</h3>
                <pre><code>{{.Info.BaseURL}}</code></pre>

                <h3 class="section-title">Constraints</h3>
                <ul>
                    {{range .Constraints}}
                    <li>{{.}}</li>
                    {{end}}
                </ul>
            </div>

            <!-- JWT Flow Panel -->
            <div class="content-panel" id="panel-flow-jwt">
                <div class="content-header">
                    <h1>🔄 JWT Authentication Flow</h1>
                    <p>Step-by-step authentication menggunakan JWT Bearer token</p>
                </div>

                <div class="flow-steps">
                    {{range $i, $step := .FlowOverview.StepsJWT}}
                    <div class="flow-step">
                        <div class="step-number">{{add $i 1}}</div>
                        <div class="step-content">{{.}}</div>
                    </div>
                    {{end}}
                </div>

                {{if .FlowOverview.Note}}
                <div class="alert alert-info">{{.FlowOverview.Note}}</div>
                {{end}}
            </div>

            <!-- API Key Flow Panel -->
            <div class="content-panel" id="panel-flow-apikey">
                <div class="content-header">
                    <h1>🔑 API Key Authentication Flow</h1>
                    <p>Step-by-step authentication menggunakan API Key</p>
                </div>

                <div class="flow-steps">
                    {{range $i, $step := .FlowOverview.StepsAPIKey}}
                    <div class="flow-step">
                        <div class="step-number">{{add $i 1}}</div>
                        <div class="step-content">{{.}}</div>
                    </div>
                    {{end}}
                </div>

                {{if .FlowOverview.Note}}
                <div class="alert alert-info">{{.FlowOverview.Note}}</div>
                {{end}}
            </div>

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
                        <pre><code>{{$ep.ExampleBody}}</code></pre>
                    </div>
                    {{end}}

                    {{if $ep.ExampleResponse}}
                    <div class="code-block">
                        <div class="code-header">Example Response</div>
                        <pre><code>{{$ep.ExampleResponse}}</code></pre>
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
                                        <div class="auth-option">
                                            <input type="radio" name="auth-method-{{$si}}-{{$ei}}" value="jwt" id="auth-jwt-{{$si}}-{{$ei}}" checked onchange="toggleInlineAuthMethod({{$si}}, {{$ei}})">
                                            <label for="auth-jwt-{{$si}}-{{$ei}}">JWT</label>
                                        </div>
                                        <div class="auth-option">
                                            <input type="radio" name="auth-method-{{$si}}-{{$ei}}" value="apikey" id="auth-apikey-{{$si}}-{{$ei}}" onchange="toggleInlineAuthMethod({{$si}}, {{$ei}})">
                                            <label for="auth-apikey-{{$si}}-{{$ei}}">API Key</label>
                                        </div>
                                        <div class="auth-option">
                                            <input type="radio" name="auth-method-{{$si}}-{{$ei}}" value="none" id="auth-none-{{$si}}-{{$ei}}" onchange="toggleInlineAuthMethod({{$si}}, {{$ei}})">
                                            <label for="auth-none-{{$si}}-{{$ei}}">Public</label>
                                        </div>
                                    </div>
                                </div>
                                <div class="form-group" id="jwt-group-{{$si}}-{{$ei}}">
                                    <label>JWT Token</label>
                                    <input type="text" class="inline-token" data-section="{{$si}}" data-endpoint="{{$ei}}" placeholder="Bearer eyJhbGci...">
                                </div>
                                <div class="form-group" id="apikey-group-{{$si}}-{{$ei}}" style="display:none">
                                    <label>API Key</label>
                                    <input type="text" class="inline-apikey" data-section="{{$si}}" data-endpoint="{{$ei}}" placeholder="your-api-key">
                                </div>
                            </div>
                            <div class="tester-row">
                                <select class="method-select inline-method" data-section="{{$si}}" data-endpoint="{{$ei}}">
                                    <option value="GET"{{if eq $ep.Method "GET"}} selected{{end}}>GET</option>
                                    <option value="POST"{{if eq $ep.Method "POST"}} selected{{end}}>POST</option>
                                    <option value="PATCH"{{if eq $ep.Method "PATCH"}} selected{{end}}>PATCH</option>
                                    <option value="DELETE"{{if eq $ep.Method "DELETE"}} selected{{end}}>DELETE</option>
                                </select>
                                <input type="text" class="url-input inline-url" data-section="{{$si}}" data-endpoint="{{$ei}}" value="{{$.Info.BaseURL}}{{$ep.Path}}">
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

            <!-- File Upload Panel -->
            <div class="content-panel" id="panel-file-upload">
                <div class="content-header">
                    <h1>📤 File Upload Flow</h1>
                    <p>Cara upload file ke Museum Service</p>
                </div>

                <p>Museum Service tidak menangani upload file langsung. Frontend harus upload ke <strong>Media Service</strong> terlebih dahulu, kemudian kirim URL yang didapat ke Museum Service.</p>

                {{range .Sections}}{{if eq .ID "file_upload"}}{{range .Flow}}
                <h3 class="section-title">Step {{.Step}}: {{.Title}}</h3>
                {{if .Endpoint}}
                <div class="endpoint-detail">
                    <div class="endpoint-header">
                        <span class="method {{lower .Endpoint.Method}}">{{.Endpoint.Method}}</span>
                        <span class="endpoint-path">{{.Endpoint.Path}}</span>
                    </div>
                    <p>Service: {{.Endpoint.Service}}</p>
                    {{if .CurlExample}}
                    <div class="code-block">
                        <div class="code-header">cURL Example</div>
                        <pre><code>{{.CurlExample}}</code></pre>
                    </div>
                    {{end}}
                    {{if .ResponseExample}}
                    <div class="code-block">
                        <div class="code-header">Response Example</div>
                        <pre><code>{{.ResponseExample}}</code></pre>
                    </div>
                    {{end}}
                </div>
                {{end}}
                {{if .Actions}}
                <p>Gunakan <code>url</code> dari response Media Service untuk update image:</p>
                <pre><code>// Update museum image
curl -X POST {{$.Info.BaseURL}}/api/v1/museum/image \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"image_url": "https://media.museumdigi.id/media/abc123/photo.jpg"}'

// Update artifact image
curl -X POST {{$.Info.BaseURL}}/api/v1/artifacts/{id}/image \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"image_url": "https://media.museumdigi.id/media/abc123/photo.jpg"}'</code></pre>
                {{end}}
                {{end}}{{end}}{{end}}

                <div class="alert alert-info">
                    <strong>Tips:</strong> Simpan juga <code>media_id</code> di database jika perlu referensi ke file di Media Service.
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
            <p>© 2026 Museum Digital Indonesia. Built with ❤️ for preserving culture.</p>
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
        // Quick Tests Data
        const quickTestsData = JSON.parse({{.APITesterDefaults.QuickTests | json}});
        const quickTestsArray = Array.isArray(quickTestsData) ? quickTestsData : Object.values(quickTestsData);
        const quickTests = {};
        quickTestsArray.forEach(t => { if (t && t.id) quickTests[t.id] = t; });

        // Load saved tokens
        document.addEventListener('DOMContentLoaded', function() {
            const savedToken = localStorage.getItem('museum_api_token');
            const savedApiKey = localStorage.getItem('museum_api_key');

            // Populate all inline token inputs
            if (savedToken) {
                document.querySelectorAll('.inline-token').forEach(el => el.value = savedToken);
            }
            if (savedApiKey) {
                document.querySelectorAll('.inline-apikey').forEach(el => el.value = savedApiKey);
            }

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
            } else {
                tester.style.display = 'none';
            }
        }

        // Toggle inline auth method
        function toggleInlineAuthMethod(sectionIdx, endpointIdx) {
            const authMethod = document.querySelector('input[name="auth-method-' + sectionIdx + '-' + endpointIdx + '"]:checked').value;
            const jwtGroup = document.getElementById('jwt-group-' + sectionIdx + '-' + endpointIdx);
            const apikeyGroup = document.getElementById('apikey-group-' + sectionIdx + '-' + endpointIdx);

            if (authMethod === 'jwt') {
                jwtGroup.style.display = 'block';
                apikeyGroup.style.display = 'none';
            } else if (authMethod === 'apikey') {
                jwtGroup.style.display = 'none';
                apikeyGroup.style.display = 'block';
            } else {
                jwtGroup.style.display = 'none';
                apikeyGroup.style.display = 'none';
            }
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
            const token = tester.querySelector('.inline-token').value;
            const apiKey = tester.querySelector('.inline-apikey').value;
            const body = tester.querySelector('.inline-body').value;
            const statusEl = tester.querySelector('.inline-status');
            const responseEl = tester.querySelector('.inline-response');

            // Save to localStorage
            if (token) localStorage.setItem('museum_api_token', token);
            if (apiKey) localStorage.setItem('museum_api_key', apiKey);

            // Build headers
            const headers = {};
            if (authMethod === 'jwt' && token) {
                headers['Authorization'] = token.startsWith('Bearer ') ? token : 'Bearer ' + token;
            } else if (authMethod === 'apikey' && apiKey) {
                headers['X-API-Key'] = apiKey;
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
                    responseEl.innerHTML = '<code>' + JSON.stringify(data, null, 2) + '</code>';
                } else {
                    responseEl.innerHTML = '<code>' + data + '</code>';
                }
            } catch (error) {
                statusEl.textContent = 'Error';
                statusEl.className = 'status-badge inline-status status-error';
                responseEl.innerHTML = '<code>Error: ' + error.message + '</code>';
            }
        }

        // Load quick test
        function loadQuickTest(testId) {
            const test = quickTests[testId];
            if (!test) return;

            // Find the endpoint panel and show it
            document.querySelectorAll('.content-panel').forEach(el => {
                if (el.id.startsWith('panel-endpoint')) {
                    const urlInput = el.querySelector('.inline-url');
                    if (urlInput && test.url && urlInput.value.indexOf(test.url.replace('{{"{{.Info.BaseURL}}" | js}}', '')) !== -1) {
                        showPanel(el.id);
                        el.querySelector('.inline-method').value = test.method;
                        if (test.body) {
                            el.querySelector('.inline-body').value = typeof test.body === 'object' ? JSON.stringify(test.body, null, 2) : test.body;
                        }
                        const testerBtn = el.querySelector('.try-it-btn');
                        if (testerBtn) testerBtn.click();
                    }
                }
            });

            if (window.innerWidth <= 768) {
                toggleSidebar();
            }
        }
    </script>
</body>
</html>
`