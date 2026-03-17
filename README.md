# Memory Flow

面向研发团队的统一项目管理平台，支持项目信息沉淀、需求与缺陷流转、状态追踪，以及 AI 协作所需的 Memory 管理能力。

## 核心功能

- **项目管理** — 项目元信息维护（Git、CI/CD、文档地址、设计原则等）
- **需求 / Bug 管理** — 统一工作项模型，支持优先级（P0/P1/P2）、状态流转、标签、关联 Git/PR
- **进度管理** — 看板视图（拖拽）、状态/优先级统计图表、趋势分析
- **Memory 管理** — Recall / Write 两类记录，支持关联项目和工作项，为 AI/Agent 协作提供数据基础

## 技术栈

| 层 | 技术 |
|---|---|
| 前端 | React + TypeScript + Vite + Ant Design + React Query + recharts + dnd-kit |
| 后端 | Go + chi + pgx |
| 数据库 | PostgreSQL 16 |
| 认证 | JWT + bcrypt |

## 快速开始

### 前置要求

- Go 1.23+
- Node.js 18+
- PostgreSQL 16+

### 1. 创建数据库

```bash
psql -h <host> -U <user> -d postgres -c "CREATE DATABASE memory_flow;"
```

### 2. 启动后端

```bash
cd backend
go mod download

DATABASE_URL="postgres://<user>@<host>:5432/memory_flow?sslmode=disable" \
JWT_SECRET="your-secret" \
PORT="8080" \
go run ./cmd/server
```

启动时会自动执行数据库迁移。

### 3. 启动前端

```bash
cd frontend
npm install
npm run dev
```

前端默认运行在 `http://localhost:5173`，API 请求代理到 `http://localhost:8080`。

### 4. 创建用户

首次使用需手动插入用户（密码需要 bcrypt 加密）：

```bash
# 生成密码哈希
python3 -c "import bcrypt; print(bcrypt.hashpw(b'your-password', bcrypt.gensalt()).decode())"

# 插入用户
psql -h <host> -U <user> -d memory_flow -c \
  "INSERT INTO users (username, password_hash, display_name, role) VALUES ('admin', '<hash>', '管理员', 'admin');"
```

## 项目结构

```
memory_flow/
├── backend/
│   ├── cmd/server/          # 入口
│   ├── internal/
│   │   ├── config/          # 配置加载
│   │   ├── database/        # 数据库连接
│   │   ├── handler/         # HTTP 处理器
│   │   ├── middleware/      # 中间件（JWT、CORS、日志）
│   │   ├── model/           # 数据模型
│   │   ├── repository/      # 数据访问层
│   │   └── service/         # 业务逻辑层
│   └── migrations/          # SQL 迁移文件
└── frontend/
    └── src/
        ├── api/             # API 客户端
        ├── components/      # 通用组件
        ├── hooks/           # React Hooks
        ├── pages/           # 页面
        └── types/           # TypeScript 类型定义
```

## API 概览

所有接口在 `/api/v1/` 下，需携带 JWT Token（`Authorization: Bearer <token>`）。

| 模块 | 端点 | 说明 |
|------|------|------|
| 认证 | `POST /auth/login` | 登录获取 Token |
| 项目 | `GET/POST /projects` | 项目列表 / 创建 |
| 项目 | `GET/PUT/DELETE /projects/:id` | 详情 / 更新 / 归档 |
| 工作项 | `GET/POST /projects/:id/issues` | 工作项列表 / 创建 |
| 工作项 | `GET/PUT /issues/:id` | 详情 / 更新 |
| 工作项 | `PATCH /issues/:id/status` | 状态流转 |
| 进度 | `GET /projects/:id/progress/summary` | 统计概览 |
| 进度 | `GET /projects/:id/progress/trend` | 趋势数据 |
| 标签 | `GET/POST /tags` | 标签管理 |
| Memory | `GET/POST /memories` | Memory 列表 / 创建 |
| Memory | `GET/PUT/DELETE /memories/:id` | 详情 / 更新 / 删除 |

## 状态流转

```
Todo → In Progress → Review → Testing → Done → Closed
  ↓                    ↓         ↓         ↓
Rejected ←─────────────┴─────────┴─────────┘
  ↓
 Todo
```

## License

[MIT](LICENSE)
