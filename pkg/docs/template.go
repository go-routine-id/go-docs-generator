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
            --primary: #4f46e5;
            --primary-dark: #4338ca;
            --secondary: #06b6d4;
            --success: #10b981;
            --warning: #f59e0b;
            --danger: #ef4444;
            --dark: #1f2937;
            --gray-100: #f3f4f6;
            --gray-200: #e5e7eb;
            --gray-300: #d1d5db;
            --gray-600: #4b5563;
            --gray-700: #374151;
            --gray-800: #1f2937;
            --gray-900: #111827;
            --bg: #0f172a;
            --card-bg: #1e293b;
            --text: #f1f5f9;
            --text-muted: #94a3b8;
            --border: #334155;
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

        /* Header */
        .header {
            background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dark) 100%);
            padding: 3rem 2rem;
            text-align: center;
            position: relative;
            overflow: hidden;
        }

        .header::before {
            content: '';
            position: absolute;
            top: -50%;
            left: -50%;
            width: 200%;
            height: 200%;
            background: radial-gradient(circle, rgba(255,255,255,0.1) 1px, transparent 1px);
            background-size: 20px 20px;
            opacity: 0.3;
        }

        .header h1 {
            font-size: 2.5rem;
            font-weight: 700;
            margin-bottom: 0.5rem;
            position: relative;
        }

        .header p {
            font-size: 1.1rem;
            opacity: 0.9;
            position: relative;
        }

        .header-actions {
            margin-top: 1.5rem;
            position: relative;
        }

        .btn {
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
            padding: 0.75rem 1.5rem;
            border-radius: 8px;
            font-weight: 500;
            text-decoration: none;
            transition: all 0.2s;
            border: none;
            cursor: pointer;
        }

        .btn-secondary {
            background: var(--card-bg);
            color: var(--text);
            border: 1px solid var(--border);
        }

        .btn-secondary:hover {
            background: var(--bg);
            border-color: var(--primary);
            color: var(--primary);
        }

        /* Navigation */
        .nav {
            background: var(--card-bg);
            border-bottom: 1px solid var(--border);
            padding: 1rem 2rem;
            position: sticky;
            top: 0;
            z-index: 100;
            display: flex;
            gap: 2rem;
            overflow-x: auto;
        }

        .nav a {
            color: var(--text-muted);
            text-decoration: none;
            font-weight: 500;
            padding: 0.5rem 1rem;
            border-radius: 6px;
            transition: all 0.2s;
            white-space: nowrap;
        }

        .nav a:hover, .nav a.active {
            color: var(--primary);
            background: rgba(79, 70, 229, 0.1);
        }

        /* Container */
        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 2rem;
        }

        /* Sections */
        .section {
            background: var(--card-bg);
            border-radius: 12px;
            padding: 2rem;
            margin-bottom: 2rem;
            border: 1px solid var(--border);
        }

        .section h2 {
            color: var(--primary);
            font-size: 1.8rem;
            margin-bottom: 1rem;
        }

        .section h3 {
            color: var(--secondary);
            font-size: 1.3rem;
            margin: 1.5rem 0 0.75rem 0;
        }

        .section p {
            color: var(--text-muted);
            margin-bottom: 1rem;
        }

        /* Cards */
        .cards {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 1.5rem;
            margin: 1.5rem 0;
        }

        .card {
            background: var(--bg);
            padding: 1.5rem;
            border-radius: 12px;
            border: 1px solid var(--border);
            transition: all 0.2s;
        }

        .card:hover {
            border-color: var(--primary);
            transform: translateY(-2px);
        }

        .card h4 {
            color: var(--text);
            margin-bottom: 0.5rem;
        }

        .card p {
            color: var(--text-muted);
            font-size: 0.9rem;
            margin: 0;
        }

        /* Code blocks */
        pre {
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 8px;
            padding: 1rem;
            overflow-x: auto;
            font-family: 'Fira Code', monospace;
            font-size: 0.9rem;
            color: var(--text);
            margin: 1rem 0;
        }

        code {
            font-family: 'Fira Code', monospace;
            background: var(--bg);
            padding: 0.2rem 0.4rem;
            border-radius: 4px;
            font-size: 0.9rem;
            color: var(--secondary);
        }

        pre code {
            padding: 0;
            background: none;
        }

        /* Alert */
        .alert {
            padding: 1rem;
            border-radius: 8px;
            margin: 1rem 0;
        }

        .alert-info {
            background: rgba(6, 182, 212, 0.1);
            border: 1px solid var(--secondary);
        }

        /* Flow Diagram */
        #flow-diagram-container {
            width: 100%;
            height: 500px;
            background: var(--bg);
            border-radius: 8px;
            border: 1px solid var(--border);
        }

        /* Endpoint */
        .endpoint {
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 8px;
            padding: 1.5rem;
            margin: 1rem 0;
        }

        .endpoint-header {
            display: flex;
            align-items: center;
            gap: 1rem;
            margin-bottom: 0.75rem;
            flex-wrap: wrap;
        }

        .method {
            padding: 0.25rem 0.75rem;
            border-radius: 4px;
            font-weight: 600;
            font-size: 0.85rem;
            text-transform: uppercase;
        }

        .method.get { background: rgba(6, 182, 212, 0.2); color: var(--secondary); }
        .method.post { background: rgba(16, 185, 129, 0.2); color: var(--success); }
        .method.patch { background: rgba(245, 158, 11, 0.2); color: var(--warning); }
        .method.delete { background: rgba(239, 68, 68, 0.2); color: var(--danger); }

        .endpoint-path {
            font-family: 'Fira Code', monospace;
            color: var(--text);
            font-weight: 500;
        }

        /* Table */
        table {
            width: 100%;
            border-collapse: collapse;
            margin: 1rem 0;
            font-size: 0.9rem;
        }

        th, td {
            padding: 0.75rem;
            text-align: left;
            border-bottom: 1px solid var(--border);
        }

        th {
            color: var(--text);
            font-weight: 600;
        }

        td {
            color: var(--text-muted);
        }

        /* Badge */
        .badge {
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.75rem;
            font-weight: 600;
        }

        .badge.required {
            background: rgba(239, 68, 68, 0.2);
            color: var(--danger);
        }

        .badge.optional {
            background: rgba(148, 163, 184, 0.2);
            color: var(--text-muted);
        }

        /* API Tester */
        .api-tester {
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 8px;
            padding: 1.5rem;
        }

        .tester-row {
            display: flex;
            gap: 0.75rem;
            margin-bottom: 1rem;
        }

        .method-select {
            padding: 0.6rem;
            background: var(--card-bg);
            border: 1px solid var(--border);
            border-radius: 6px;
            color: var(--text);
            font-weight: 600;
        }

        .url-input {
            flex: 1;
            padding: 0.6rem 0.75rem;
            background: var(--card-bg);
            border: 1px solid var(--border);
            border-radius: 6px;
            color: var(--text);
        }

        .send-btn {
            padding: 0.6rem 1.25rem;
            background: var(--primary);
            color: white;
            border: none;
            border-radius: 6px;
            font-weight: 500;
            cursor: pointer;
            transition: background 0.2s;
        }

        .send-btn:hover {
            background: var(--primary-dark);
        }

        .tester-tabs {
            display: flex;
            gap: 0.5rem;
            margin-bottom: 1rem;
            border-bottom: 1px solid var(--border);
        }

        .tab-btn {
            padding: 0.6rem 1rem;
            background: none;
            border: none;
            color: var(--text-muted);
            cursor: pointer;
            border-bottom: 2px solid transparent;
            transition: all 0.2s;
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
            padding: 0.5rem;
            background: var(--card-bg);
            border: 1px solid var(--border);
            border-radius: 6px;
            color: var(--text);
        }

        .icon-btn {
            width: 36px;
            height: 36px;
            border-radius: 6px;
            border: none;
            background: var(--primary);
            color: white;
            font-size: 1.2rem;
            cursor: pointer;
        }

        .icon-btn.danger {
            background: var(--danger);
        }

        textarea {
            width: 100%;
            min-height: 150px;
            padding: 0.75rem;
            background: var(--card-bg);
            border: 1px solid var(--border);
            border-radius: 6px;
            color: var(--text);
            font-family: 'Fira Code', monospace;
            font-size: 0.9rem;
            resize: vertical;
        }

        .response-section {
            margin-top: 1rem;
            border: 1px solid var(--border);
            border-radius: 6px;
            overflow: hidden;
        }

        .response-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 0.75rem;
            background: var(--card-bg);
            border-bottom: 1px solid var(--border);
        }

        .response-body {
            padding: 1rem;
            margin: 0;
            max-height: 400px;
            overflow: auto;
        }

        .status-badge {
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.85rem;
            font-weight: 600;
        }

        .status-success { background: rgba(16, 185, 129, 0.2); color: var(--success); }
        .status-error { background: rgba(239, 68, 68, 0.2); color: var(--danger); }

        /* Quick Tests */
        .quick-tests {
            display: flex;
            flex-wrap: wrap;
            gap: 0.5rem;
            margin-top: 1rem;
        }

        .quick-test-btn {
            padding: 0.5rem 1rem;
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 6px;
            color: var(--text);
            cursor: pointer;
            transition: all 0.2s;
        }

        .quick-test-btn:hover {
            border-color: var(--primary);
            background: rgba(79, 70, 229, 0.1);
        }

        /* Form Group */
        .form-group {
            margin-bottom: 1rem;
        }

        .form-group label {
            display: block;
            margin-bottom: 0.5rem;
            font-weight: 500;
            color: var(--text);
        }

        .form-group input {
            width: 100%;
            padding: 0.6rem 0.75rem;
            background: var(--card-bg);
            border: 1px solid var(--border);
            border-radius: 6px;
            color: var(--text);
        }

        .form-group small {
            display: block;
            margin-top: 0.25rem;
            color: var(--text-muted);
            font-size: 0.85rem;
        }

        /* Footer */
        .footer {
            background: var(--card-bg);
            border-top: 1px solid var(--border);
            padding: 2rem;
            text-align: center;
            color: var(--text-muted);
        }

        .footer a {
            color: var(--primary);
            text-decoration: none;
        }

        .footer a:hover {
            text-decoration: underline;
        }

        /* Responsive */
        @media (max-width: 768px) {
            .header h1 {
                font-size: 1.8rem;
            }

            .tester-row {
                flex-direction: column;
            }

            .nav {
                gap: 1rem;
            }

            #flow-diagram-container {
                height: 350px;
            }
        }
    </style>
</head>
<body>
    <header class="header">
        <h1>🏛️ {{.Info.Title}}</h1>
        <p>{{.Info.Description}}</p>
        <div class="header-actions">
            <a href="/api/docs/yaml" class="btn btn-secondary" download>
                📥 Download API Spec (YAML)
            </a>
        </div>
    </header>

    <nav class="nav">
        <a href="#overview" class="active">Overview</a>
        <a href="#flow-diagram">🔄 Flow</a>
        <a href="#endpoints">Endpoints</a>
        <a href="#file-upload">📤 Upload</a>
        <a href="#api-tester">🧪 API Tester</a>
    </nav>

    <div class="container">
        <!-- Overview Section -->
        <section class="section" id="overview">
            <h2>📋 Overview</h2>
            <p>{{.Info.Description}}</p>

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

            <h3>Base URL</h3>
            <pre><code>{{.Info.BaseURL}}</code></pre>

            <h3>Authentication</h3>
            <p>Type: <strong>{{.Authentication.Type}}</strong></p>
            <p>Header: <code>{{.Authentication.Header}}: Bearer &lt;token&gt;</code></p>
            <p>Token source: {{.Authentication.Source}}</p>
            <div class="alert alert-info">
                <strong>Token contains:</strong> {{join .Authentication.TokenContains ", "}}
            </div>

            <h3>Flow Overview</h3>
            <ol>
                {{range .FlowOverview.Steps}}
                <li>{{.}}</li>
                {{end}}
            </ol>

            <h3>Constraints</h3>
            <ul>
                {{range .Constraints}}
                <li>{{.}}</li>
                {{end}}
            </ul>
        </section>

        <!-- Flow Diagram Section -->
        <section class="section" id="flow-diagram">
            <h2>🔄 Service Flow Diagram</h2>
            <p>Visualisasi alur data antara Account Service → Museum → Artifacts → Media Service</p>
            <div id="flow-diagram-container"></div>
        </section>

        <!-- Endpoints Section -->
        <section class="section" id="endpoints">
            <h2>🔌 API Endpoints</h2>
            {{range .Sections}}
            {{if .Endpoints}}
            <h3>{{.Title}}</h3>
            {{if .Description}}<p>{{.Description}}</p>{{end}}
            {{range .Endpoints}}
            <div class="endpoint">
                <div class="endpoint-header">
                    <span class="method {{lower .Method}}">{{.Method}}</span>
                    <span class="endpoint-path">{{.Path}}</span>
                </div>
                <p>{{.Description}}</p>
                {{if eq .Auth "required"}}
                <p><strong>Auth:</strong> Required {{if .Permission}}<code>{{.Permission}}</code>{{end}}</p>
                {{else if eq .Auth "optional"}}
                <p><strong>Auth:</strong> Optional (public access available)</p>
                {{end}}
                {{if .QueryParams}}
                <table>
                    <tr><th>Param</th><th>Type</th><th>Required</th><th>Description</th></tr>
                    {{range .QueryParams}}
                    <tr>
                        <td>{{.Name}}</td>
                        <td>{{.Type}}</td>
                        <td><span class="badge {{if .Required}}required{{else}}optional{{end}}">{{if .Required}}required{{else}}optional{{end}}</span></td>
                        <td>{{.Description}}{{if .Default}} (default: {{.Default}}){{end}}</td>
                    </tr>
                    {{end}}
                </table>
                {{end}}
                {{if .Body}}
                <table>
                    <tr><th>Field</th><th>Type</th><th>Required</th><th>Description</th></tr>
                    {{range .Body}}
                    <tr>
                        <td>{{.Name}}</td>
                        <td>{{.Type}}</td>
                        <td><span class="badge {{if .Required}}required{{else}}optional{{end}}">{{if .Required}}required{{else}}optional{{end}}</span></td>
                        <td>{{.Description}}{{if .Example}} <code>ex: {{.Example}}</code>{{end}}</td>
                    </tr>
                    {{end}}
                </table>
                {{end}}
                {{if .ExampleBody}}
                <p><strong>Request Body:</strong></p>
                <pre><code>{{.ExampleBody}}</code></pre>
                {{end}}
                {{if .ExampleResponse}}
                <p><strong>Response Example:</strong></p>
                <pre><code>{{.ExampleResponse}}</code></pre>
                {{end}}
            </div>
            {{end}}
            {{end}}
            {{end}}
        </section>

        <!-- File Upload Section -->
        <section class="section" id="file-upload">
            <h2>📤 File Upload Flow</h2>
            <p>Museum Service tidak menangani upload file langsung. Frontend harus upload ke <strong>Media Service</strong> terlebih dahulu, kemudian kirim URL yang didapat ke Museum Service.</p>
            {{range .Sections}}{{if eq .ID "file_upload"}}{{range .Flow}}
            <h3>Step {{.Step}}: {{.Title}}</h3>
            {{if .Endpoint}}
            <div class="endpoint">
                <div class="endpoint-header">
                    <span class="method {{lower .Endpoint.Method}}">{{.Endpoint.Method}}</span>
                    <span class="endpoint-path">{{.Endpoint.Path}}</span>
                </div>
                <p>Service: {{.Endpoint.Service}}</p>
                <p><strong>Auth:</strong> Required <code>{{.Endpoint.Permission}}</code></p>
                {{if .CurlExample}}<pre><code>{{.CurlExample}}</code></pre>{{end}}
                {{if .ResponseExample}}<p><strong>Response:</strong></p><pre><code>{{.ResponseExample}}</code></pre>{{end}}
            </div>
            {{end}}
            {{if .Actions}}
            <p>Gunakan <code>url</code> dari response Media Service untuk update image:</p>
            <pre><code>// Update museum image
curl -X POST {{$.Info.BaseURL}}/api/v1/museum/image \\
  -H "Authorization: Bearer TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"image_url": "https://media.museumdigi.id/media/abc123/photo.jpg"}'

// Update artifact image
curl -X POST {{$.Info.BaseURL}}/api/v1/artifacts/{id}/image \\
  -H "Authorization: Bearer TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"image_url": "https://media.museumdigi.id/media/abc123/photo.jpg"}'</code></pre>
            {{end}}
            {{end}}{{end}}{{end}}
            <div class="alert alert-info">
                <strong>Tips:</strong> Simpan juga <code>media_id</code> di database jika perlu referensi ke file di Media Service (untuk delete atau update nanti).
            </div>
        </section>

        <!-- API Tester Section -->
        <section class="section" id="api-tester">
            <h2>🧪 API Tester</h2>
            <p>Test API endpoints directly from this documentation. Masukkan JWT token (optional) dan parameters, lalu klik "Send Request".</p>

            <div class="api-tester">
                <div class="tester-config">
                    <div class="form-group">
                        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
                            <label>JWT Token (Optional)</label>
                            <div style="display: flex; align-items: center; gap: 1rem;">
                                <label style="display: flex; align-items: center; gap: 0.5rem; font-weight: normal; font-size: 0.85rem; cursor: pointer;">
                                    <input type="checkbox" id="save-token" checked>
                                    <span>Remember Token</span>
                                </label>
                                <button onclick="clearToken()" style="background: none; border: none; color: var(--danger); font-size: 0.85rem; cursor: pointer; text-decoration: underline;">Clear</button>
                            </div>
                        </div>
                        <input type="text" id="tester-token" placeholder="Bearer eyJhbGciOiJSUzI1NiIs...">
                        <small>Token tersimpan di browser (localStorage). Leave empty for public endpoints.</small>
                    </div>
                </div>

                <div class="tester-row">
                    <select id="tester-method" class="method-select">
                        <option value="GET" selected>GET</option>
                        <option value="POST">POST</option>
                        <option value="PATCH">PATCH</option>
                        <option value="DELETE">DELETE</option>
                    </select>
                    <input type="text" id="tester-url" class="url-input" value="{{.APITesterDefaults.DefaultURL}}" placeholder="{{.APITesterDefaults.DefaultURL}}">
                    <button onclick="sendRequest()" class="send-btn">Send Request</button>
                </div>

                <div class="tester-tabs">
                    <button class="tab-btn active" onclick="switchTab('params')">Query Params</button>
                    <button class="tab-btn" onclick="switchTab('body')">JSON Body</button>
                    <button class="tab-btn" onclick="switchTab('formdata')">Form Data</button>
                    <button class="tab-btn" onclick="switchTab('headers')">Headers</button>
                </div>

                <div id="tab-params" class="tab-content active">
                    <div class="param-row">
                        <input type="text" placeholder="Key" class="param-key" value="page">
                        <input type="text" placeholder="Value" class="param-value" value="1">
                        <button onclick="addParamRow(this)" class="icon-btn">+</button>
                    </div>
                    <div class="param-row">
                        <input type="text" placeholder="Key" class="param-key" value="page_size">
                        <input type="text" placeholder="Value" class="param-value" value="9">
                        <button onclick="removeParamRow(this)" class="icon-btn danger">×</button>
                    </div>
                </div>

                <div id="tab-body" class="tab-content">
                    <textarea id="request-body" placeholder='{
  "name": "Museum Test",
  "location": "Jakarta",
  "description": "Test museum"
}'></textarea>
                </div>

                <div id="tab-formdata" class="tab-content">
                    <div class="formdata-info" style="background: var(--bg); padding: 1rem; border-radius: 8px; margin-bottom: 1rem; font-size: 0.9rem;">
                        <p style="margin: 0; color: var(--text-muted);">📤 Gunakan untuk upload file ke Media Service.</p>
                        <p style="margin: 0.5rem 0 0 0; color: var(--warning); font-size: 0.85rem;">⚠️ Note: Untuk upload file asli, gunakan curl atau Postman dengan multipart/form-data.</p>
                    </div>
                    <div id="formdata-fields">
                        <div class="formdata-row" style="display: flex; gap: 0.75rem; margin-bottom: 0.75rem; align-items: center;">
                            <input type="text" placeholder="Field name" class="formdata-key" value="folder_id" style="flex: 1; padding: 0.6rem 0.75rem; background: var(--bg); border: 1px solid var(--border); border-radius: 6px; color: var(--text);">
                            <input type="text" placeholder="Value" class="formdata-value" placeholder="optional-folder-id" style="flex: 2; padding: 0.6rem 0.75rem; background: var(--bg); border: 1px solid var(--border); border-radius: 6px; color: var(--text);">
                            <button onclick="addFormDataRow(this)" class="icon-btn" style="width: 36px; height: 36px; border-radius: 6px; border: none; background: var(--primary); color: white; font-size: 1.2rem; cursor: pointer;">+</button>
                        </div>
                    </div>
                    <div style="margin-top: 1rem; padding: 1rem; background: var(--bg); border-radius: 8px; border: 2px dashed var(--border);">
                        <label style="display: block; margin-bottom: 0.5rem; color: var(--text-muted); font-size: 0.9rem;">File Upload (Simulasi)</label>
                        <input type="file" id="formdata-file" style="width: 100%; color: var(--text);" onchange="updateFormDataFileName(this)">
                        <input type="hidden" id="formdata-file-field" value="file">
                    </div>
                </div>

                <div id="tab-headers" class="tab-content">
                    <div class="header-row">
                        <input type="text" value="Content-Type" class="header-key" readonly>
                        <input type="text" value="application/json" class="header-value">
                    </div>
                </div>

                <div class="response-section">
                    <div class="response-header">
                        <span>Response</span>
                        <span id="response-status" class="status-badge"></span>
                    </div>
                    <pre id="response-body" class="response-body"><code>Click "Send Request" to see the response...</code></pre>
                </div>
            </div>

            <h3>Quick Test Buttons</h3>
            {{range $group, $tests := groupTests .APITesterDefaults.QuickTests}}
            <h4 style="margin-top: 1rem;">{{$group}}</h4>
            <div class="quick-tests">
                {{range $tests}}
                <button onclick="loadTest('{{.ID}}')" class="quick-test-btn"{{if .IsFormData}} title="Upload file ke Media Service dulu"{{end}}>
                    <span class="method {{lower .Method}}">{{.Method}}</span> {{.Label}}
                </button>
                {{end}}
            </div>
            {{end}}
        </section>
    </div>

    <footer class="footer">
        <p>© 2026 Museum Digital Indonesia. Built with ❤️ for preserving culture.</p>
        <p style="margin-top: 0.5rem; font-size: 0.9rem;">
            <a href="/swagger/index.html">Swagger UI</a> •
            <a href="/swagger/doc.json">OpenAPI JSON</a> •
            <a href="/api/docs/yaml" download>📥 API Spec YAML (for AI)</a> •
            <a href="/api/docs/spec">🤖 AI Spec (JSON)</a>
        </p>
    </footer>

    <!-- ReactFlow Component -->
    <script type="text/babel">
        // ReactFlow v11 UMD exports - komponen ada di window.ReactFlow.ReactFlow
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
                {{if .Style}},type: 'dashed'{{end}}
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
        // Quick Tests Data - convert to object map for easy lookup
        const quickTestsData = JSON.parse({{.APITesterDefaults.QuickTests | json}});
        const quickTestsArray = Array.isArray(quickTestsData) ? quickTestsData : Object.values(quickTestsData);
        const quickTests = {};
        quickTestsArray.forEach(t => { if (t && t.id) quickTests[t.id] = t; });

        // Load saved token
        document.addEventListener('DOMContentLoaded', function() {
            const savedToken = localStorage.getItem('museum_api_token');
            if (savedToken) {
                document.getElementById('tester-token').value = savedToken;
            }
        });

        function saveToken() {
            const token = document.getElementById('tester-token').value;
            if (document.getElementById('save-token').checked && token) {
                localStorage.setItem('museum_api_token', token);
            }
        }

        function clearToken() {
            document.getElementById('tester-token').value = '';
            localStorage.removeItem('museum_api_token');
        }

        function switchTab(tabName) {
            document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
            document.querySelectorAll('.tab-btn').forEach(el => el.classList.remove('active'));
            document.getElementById('tab-' + tabName).classList.add('active');
            event.target.classList.add('active');
        }

        function addParamRow(btn) {
            const row = btn.parentElement;
            const newRow = row.cloneNode(true);
            newRow.querySelector('.param-key').value = '';
            newRow.querySelector('.param-value').value = '';
            newRow.querySelector('button').textContent = '×';
            newRow.querySelector('button').classList.add('danger');
            newRow.querySelector('button').onclick = function() { removeParamRow(this); };
            row.parentElement.appendChild(newRow);
        }

        function removeParamRow(btn) {
            btn.parentElement.remove();
        }

        function addFormDataRow(btn) {
            const row = btn.parentElement;
            const newRow = row.cloneNode(true);
            newRow.querySelector('.formdata-key').value = '';
            newRow.querySelector('.formdata-value').value = '';
            newRow.querySelector('button').onclick = function() { removeFormDataRow(this); };
            row.parentElement.appendChild(newRow);
        }

        function removeFormDataRow(btn) {
            btn.parentElement.remove();
        }

        function updateFormDataFileName(input) {
            if (input.files && input.files[0]) {
                console.log('Selected file:', input.files[0].name);
            }
        }

        async function sendRequest() {
            const method = document.getElementById('tester-method').value;
            let url = document.getElementById('tester-url').value;
            const token = document.getElementById('tester-token').value;
            const responseBody = document.getElementById('response-body');
            const statusBadge = document.getElementById('response-status');

            if (!url.startsWith('http')) {
                url = '{{.Info.BaseURL}}' + url;
            }

            // Save token if checked
            saveToken();

            // Build query params
            const params = new URLSearchParams();
            document.querySelectorAll('#tab-params .param-row').forEach(row => {
                const key = row.querySelector('.param-key').value;
                const value = row.querySelector('.param-value').value;
                if (key && value) params.append(key, value);
            });
            if (params.toString()) url += (url.includes('?') ? '&' : '?') + params.toString();

            // Build headers
            const headers = {};
            if (token) headers['Authorization'] = token.startsWith('Bearer ') ? token : 'Bearer ' + token;

            const options = {
                method: method,
                headers: headers
            };

            // Add body for non-GET requests
            if (method !== 'GET') {
                const activeTab = document.querySelector('.tab-content.active').id;
                if (activeTab === 'tab-body') {
                    const body = document.getElementById('request-body').value;
                    if (body) {
                        headers['Content-Type'] = 'application/json';
                        options.body = body;
                    }
                } else if (activeTab === 'tab-formdata') {
                    const formData = new FormData();
                    document.querySelectorAll('.formdata-row').forEach(row => {
                        const key = row.querySelector('.formdata-key').value;
                        const value = row.querySelector('.formdata-value').value;
                        if (key) formData.append(key, value);
                    });
                    const fileInput = document.getElementById('formdata-file');
                    if (fileInput.files[0]) {
                        formData.append('file', fileInput.files[0]);
                    }
                    options.body = formData;
                }
            }

            responseBody.innerHTML = '<code>Sending request...</code>';
            statusBadge.className = 'status-badge';

            try {
                const response = await fetch(url, options);
                const contentType = response.headers.get('content-type');
                let data;
                if (contentType && contentType.includes('application/json')) {
                    data = await response.json();
                } else {
                    data = await response.text();
                }

                statusBadge.textContent = response.status + ' ' + response.statusText;
                statusBadge.className = 'status-badge ' + (response.ok ? 'status-success' : 'status-error');

                if (typeof data === 'object') {
                    responseBody.innerHTML = '<code>' + JSON.stringify(data, null, 2) + '</code>';
                } else {
                    responseBody.innerHTML = '<code>' + data + '</code>';
                }
            } catch (error) {
                statusBadge.textContent = 'Error';
                statusBadge.className = 'status-badge status-error';
                responseBody.innerHTML = '<code>Error: ' + error.message + '</code>';
            }
        }

        function loadTest(testId) {
            const test = quickTests[testId];
            if (!test) return;

            document.getElementById('tester-method').value = test.method;
            document.getElementById('tester-url').value = test.url;

            // Set body if present
            if (test.body && test.body !== '') {
                if (typeof test.body === 'object') {
                    document.getElementById('request-body').value = JSON.stringify(test.body, null, 2);
                } else {
                    document.getElementById('request-body').value = test.body;
                }
                switchTab('body');
            }
        }

        // Navigation highlight
        document.querySelectorAll('.nav a').forEach(link => {
            link.addEventListener('click', function() {
                document.querySelectorAll('.nav a').forEach(l => l.classList.remove('active'));
                this.classList.add('active');
            });
        });
    </script>
</body>
</html>
`
