# gocron - Distributed scheduled Task Scheduler

[![Release](https://img.shields.io/github/release/gocronx-team/gocron.svg?label=Release)](https://github.com/gocronx-team/gocron/releases) [![Downloads](https://img.shields.io/github/downloads/gocronx-team/gocron/total.svg)](https://github.com/gocronx-team/gocron/releases) [![License](https://img.shields.io/github/license/gocronx-team/gocron.svg)](https://github.com/gocronx-team/gocron/blob/master/LICENSE)

English | [简体中文](README_ZH.md)

A lightweight distributed scheduled task management system developed in Go, designed to replace Linux-crontab.

## 📖 Documentation

Full documentation is available at: **[document](https://gocron-docs.pages.dev/en/)**

- 🚀 [Quick Start](https://gocron-docs.pages.dev/en/guide/quick-start) - Installation and deployment guide
- 🤖 [Agent Auto-Registration](https://gocron-docs.pages.dev/en/guide/agent-registration) - One-click task node deployment
- ⚙️ [Configuration](https://gocron-docs.pages.dev/en/guide/configuration) - Detailed configuration guide
- 🔌 [API Documentation](https://gocron-docs.pages.dev/en/guide/api) - API reference

## ✨ Features

- **Web Interface**: Intuitive task management interface
- **Second-level Precision**: Supports Crontab expressions with second precision
- **High Availability**: Database-lock-based leader election, automatic failover in seconds
- **Task Retry**: Configurable retry policies for failed tasks
- **Task Dependency**: Supports task dependency configuration
- **Access Control**: Comprehensive user and permission management
- **2FA Security**: Two-Factor Authentication support
- **Agent Auto-Registration**: One-click installation for Linux/macOS
- **Multi-Database**: MySQL / PostgreSQL / SQLite support
- **Log Management**: Complete execution logs with auto-cleanup
- **Notifications**: Email, Slack, Webhook support

## 🚀 Quick Start (Docker)

The easiest way to deploy is using Docker Compose:

```bash
# 1. Clone the project
git clone https://github.com/gocronx-team/gocron.git
cd gocron

# 2. Start services
docker-compose up -d

# 3. Access Web Interface
# http://localhost:5920
```

For more deployment methods (Binary, Development), please refer to the [Installation Guide](https://gocron-docs.pages.dev/en/guide/quick-start).

## 🔷 High Availability (Optional)

gocron supports multi-instance deployment with automatic leader election. Only the leader node runs the scheduler; followers stay on hot standby and take over within seconds if the leader goes down.

### How It Works

- Uses database row locking (`SELECT ... FOR UPDATE`) for leader election — no external dependencies needed
- Leader renews its lease every 5 seconds (15-second expiry)
- Automatic failover: if the leader crashes, a follower takes over within 15 seconds
- Graceful shutdown releases the lock immediately for instant failover
- **Requires MySQL or PostgreSQL** — SQLite does not support multi-instance access, leader election is skipped automatically in SQLite mode

### Setup

1. Complete the web installation wizard on the first node
2. Copy the `.gocron/conf/` directory (app.ini, install.lock) to other nodes
3. Start all nodes pointing to the **same MySQL/PostgreSQL database**

```bash
# Node 1 — complete web install first, then start
./gocron web --port 5920

# Node 2 — copy .gocron/conf/ from Node 1, then start
./gocron web --port 5921
```

No extra configuration needed. The `scheduler_lock` table is created automatically on first startup.

For K8s/Docker deployments, mount the same config or use environment variable overrides instead of copying files.

### Environment Variable Overrides

Database and app settings can be overridden via environment variables (useful for K8s/Docker):

| Variable | Overrides |
|---|---|
| `GOCRON_DB_ENGINE` / `HOST` / `PORT` / `USER` / `PASSWORD` / `DATABASE` / `PREFIX` | Database config |
| `GOCRON_AUTH_SECRET` | JWT auth secret |

## 📸 Screenshots

<p align="center">
  <b>Scheduled Tasks</b><br>
  <img src="assets/screenshot/scheduler_en.png" alt="Scheduled Tasks" width="100%">
</p>

<table>
  <tr>
    <td width="50%" align="center"><b>Agent Auto-Registration</b></td>
    <td width="50%" align="center"><b>Task Management</b></td>
  </tr>
  <tr>
    <td><img src="assets/screenshot/agent_en.png" alt="Agent Auto-Registration" width="100%"></td>
    <td><img src="assets/screenshot/task_en.png" alt="Task Management" width="100%"></td>
  </tr>
</table>

<table>
  <tr>
    <td width="50%" align="center"><b>Statistics</b></td>
    <td width="50%" align="center"><b>Notifications</b></td>
  </tr>
  <tr>
    <td><img src="assets/screenshot/statistic_en.png" alt="Statistics" width="100%"></td>
    <td><img src="assets/screenshot/notification_en.png" alt="Notifications" width="100%"></td>
  </tr>
</table>

## 🤝 Contributing

We warmly welcome community contributions!

### How to Contribute

1. **Fork the repository**
2. **Clone your fork**

   ```bash
   git clone https://github.com/YOUR_USERNAME/gocron.git
   cd gocron
   ```

3. **Install dependencies**

   ```bash
   pnpm install
   pnpm run prepare
   ```

4. **Create a feature branch**

   ```bash
   git checkout -b feature/your-feature-name
   ```

5. **Make your changes and commit**

   ```bash
   git add .
   pnpm run commit  # Use interactive commit tool
   ```

6. **Push and create a Pull Request**
   ```bash
   git push origin feature/your-feature-name
   ```

### Commit Message Guidelines

This project uses [commitizen](https://github.com/commitizen/cz-cli) and [cz-git](https://cz-git.qbb.sh/) for standardized commit messages.

Instead of `git commit`, use:

```bash
pnpm run commit
```

This will guide you through an interactive prompt to create properly formatted commit messages like:

- `feat(task): add task dependency configuration`
- `fix(api): fix task status update issue`
- `docs: update API documentation`

### Other Ways to Contribute

- 🐛 **Report Bugs**: Please submit via GitHub Issues
- 💡 **Feature Requests**: Share your ideas through Issues
- 📝 **Documentation**: Help improve our documentation

## 📄 License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=gocronx-team/gocron&type=Date)](https://www.star-history.com/#gocronx-team/gocron&Date)
