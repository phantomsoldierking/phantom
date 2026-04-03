const featureCards = [
  {
    title: "See the whole box",
    body: "Track CPU, memory, disk pressure, active processes, listening ports, and live logs without jumping across five different tools.",
  },
  {
    title: "Inspect data at terminal speed",
    body: "Load JSON, YAML, or TOML into the Explorer tab, drill through nested structures, run gojq filters, and yank values or paths instantly.",
  },
  {
    title: "Operate where you already work",
    body: "Launch lazygit, lazydocker, kind workflows, or nvim from the same keyboard-first shell surface and stay anchored to your project directory.",
  },
];

const workflows = [
  {
    label: "Investigate",
    text: "Merge logs from files, journald units, and commands into one scrolling stream with source toggles, follow mode, and error-only filtering.",
  },
  {
    label: "Inspect",
    text: "Pivot from ports to owning processes, open env and file descriptor views, and trace state without losing context.",
  },
  {
    label: "Act",
    text: "Send HTTP requests, launch tools, run shell commands from the palette, or terminate the process that is actually causing trouble.",
  },
];

const quickFacts = [
  "10 purpose-built tabs",
  "Native Go HTTP workflows",
  "Project-aware launchers",
  "Lua configuration",
];

const commandSteps = [
"curl -L -o phantom.tar.gz https://github.com/phantomsoldierking/phantom/releases/download/v0.1.0/phantom_v0.1.0_linux_amd64.tar.gz",
"tar -xzf phantom.tar.gz",
"chmod +x phantom",
"sudo mv phantom /usr/local/bin/phantom",
"phantom --help",
];

export default function Home() {
  return (
    <main className="page-shell">
      <div className="page-glow page-glow-left" />
      <div className="page-glow page-glow-right" />

      <section className="hero">
        <div className="hero-copy">
          <p className="eyebrow">Developer operations, minus the browser sprawl</p>
          <h1>
            A keyboard-first
            <span className="hero-break" />
            ops cockpit for the terminal.
          </h1>
          <p className="hero-text">
            Phantom brings logs, processes, ports, HTTP workflows, structured payloads,
            and project launchers into one calm workspace. It is built for the messy
            middle of debugging, inspecting, and acting without losing momentum.
          </p>

          <div className="hero-note">
            <span className="hero-note-label">Built in Go</span>
            <p>
              Start in the dashboard, pivot into logs or ports, inspect a payload, fire
              a request, and launch the tool you need without leaving the same surface.
            </p>
          </div>

          <div className="hero-actions">
            <a className="button button-primary" href="#get-started">
              Run Phantom
            </a>
            <a className="button button-secondary" href="#features">
              Explore Features
            </a>
          </div>

          <ul className="fact-list" aria-label="Phantom highlights">
            {quickFacts.map((fact) => (
              <li key={fact}>{fact}</li>
            ))}
          </ul>
        </div>

        <div className="hero-panel" aria-label="Phantom interface preview">
          <div className="window-chrome">
            <span />
            <span />
            <span />
          </div>
          <div className="terminal-frame">
            <div className="terminal-header">
              <span className="prompt">phantom ./payments-api</span>
              <span className="status-pill">dashboard online</span>
            </div>

            <div className="tab-row">
              <span className="tab tab-active">Dashboard</span>
              <span className="tab">Logs</span>
              <span className="tab">Processes</span>
              <span className="tab">Ports</span>
              <span className="tab">HTTP</span>
              <span className="tab">Explorer</span>
            </div>

            <div className="panel-grid">
              <article className="panel">
                <div className="panel-title">system</div>
                <div className="metric">
                  <span>CPU</span>
                  <strong>41%</strong>
                </div>
                <div className="bar">
                  <span style={{ width: "41%" }} />
                </div>
                <div className="metric">
                  <span>Memory</span>
                  <strong>7.4 GB / 16 GB</strong>
                </div>
                <div className="bar">
                  <span style={{ width: "46%" }} />
                </div>
                <div className="metric">
                  <span>Disk</span>
                  <strong>62%</strong>
                </div>
                <div className="bar">
                  <span style={{ width: "62%" }} />
                </div>
              </article>

              <article className="panel">
                <div className="panel-title">logs</div>
                <ul className="log-list">
                  <li>
                    <span className="log-tag">api</span>
                    <span>POST /orders 201 24ms</span>
                  </li>
                  <li>
                    <span className="log-tag warn">db</span>
                    <span>slow query detected on invoices</span>
                  </li>
                  <li>
                    <span className="log-tag error">job</span>
                    <span>retrying webhook delivery attempt 2</span>
                  </li>
                  <li>
                    <span className="log-tag">proxy</span>
                    <span>127.0.0.1:8080 listening</span>
                  </li>
                </ul>
              </article>

              <article className="panel panel-wide">
                <div className="panel-title">http + explorer</div>
                <pre className="code-block">
                  <code>{`GET {{base_url}}/posts
Authorization: Bearer {{token}}

{
  "status": "ok",
  "items": [
    { "id": 42, "name": "phantom" }
  ]
}`}</code>
                </pre>
              </article>
            </div>
          </div>
        </div>
      </section>

      <section className="section" id="features">
        <div className="section-heading">
          <p className="eyebrow">Why it feels different</p>
          <h2>Phantom is built for tight investigative loops.</h2>
        </div>
        <div className="card-grid">
          {featureCards.map((card) => (
            <article className="feature-card" key={card.title}>
              <h3>{card.title}</h3>
              <p>{card.body}</p>
            </article>
          ))}
        </div>
      </section>

      <section className="section workflow-section">
        <div className="section-heading">
          <p className="eyebrow">One place to move</p>
          <h2>Start with a symptom, end with an action.</h2>
        </div>
        <div className="workflow-list">
          {workflows.map((item) => (
            <article className="workflow-card" key={item.label}>
              <span className="workflow-label">{item.label}</span>
              <p>{item.text}</p>
            </article>
          ))}
        </div>
      </section>

      <section className="section split-section">
        <div className="copy-block">
          <p className="eyebrow">Included tabs</p>
          <h2>Real workflows, not placeholder panels.</h2>
          <p>
            The current app already ships a Dashboard, Logs, Processes, Ports, HTTP,
            Explorer, Git, Docker, Kind, and Nvim surface. Missing optional binaries do
            not crash the interface; the tab stays visible and explains what is needed.
          </p>
        </div>

        <div className="feature-stack">
          <div className="stack-row">
            <span>Dashboard</span>
            <span>health, disk, top processes</span>
          </div>
          <div className="stack-row">
            <span>Logs</span>
            <span>merged sources, follow, filtering</span>
          </div>
          <div className="stack-row">
            <span>Processes + Ports</span>
            <span>PID actions, ownership tracing</span>
          </div>
          <div className="stack-row">
            <span>HTTP</span>
            <span>native transport, env templates</span>
          </div>
          <div className="stack-row">
            <span>Explorer</span>
            <span>JSON, YAML, TOML, gojq, convert</span>
          </div>
          <div className="stack-row">
            <span>Launchers</span>
            <span>lazygit, lazydocker, kind, nvim</span>
          </div>
        </div>
      </section>

      <section className="section install-section" id="get-started">
        <div className="section-heading">
          <p className="eyebrow">Get started</p>
          <h2>Build it, point it at a project, and stay in flow.</h2>
        </div>

        <div className="install-card">
          <div className="install-copy">
            <p>
              Phantom is a Go application with optional launcher integrations. Start with
              the binary, then open it in the current project directory or preload the
              Explorer tab with structured input.
            </p>
          </div>

          <pre className="command-block">
            <code>{commandSteps.join("\n")}</code>
          </pre>
        </div>
      </section>
    </main>
  );
}
