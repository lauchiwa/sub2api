.PHONY: build build-backend build-frontend test test-backend test-frontend test-frontend-critical

FRONTEND_CRITICAL_VITEST := \
	src/views/auth/__tests__/LinuxDoCallbackView.spec.ts \
	src/views/auth/__tests__/WechatCallbackView.spec.ts \
	src/views/user/__tests__/PaymentView.spec.ts \
	src/views/user/__tests__/PaymentResultView.spec.ts \
	src/components/user/profile/__tests__/ProfileInfoCard.spec.ts \
	src/views/admin/__tests__/SettingsView.spec.ts

# 一键编译前后端
build: build-backend build-frontend

# 编译后端（复用 backend/Makefile）
build-backend:
	@$(MAKE) -C backend build

# 编译前端（需要已安装依赖）
# 注意：用 `cd frontend &&` 而不是 `pnpm --dir frontend`。后者的工作目录是仓库根，
# 根目录没有 package.json，corepack 读不到 packageManager 字段会退回最新版 pnpm，
# 与 frontend/package.json 里钉死的版本冲突而报错。
build-frontend:
	@cd frontend && pnpm run build

# 运行测试（后端 + 前端）
test: test-backend test-frontend

test-backend:
	@$(MAKE) -C backend test

test-frontend:
	@cd frontend && pnpm run lint:check
	@cd frontend && pnpm run typecheck
	@$(MAKE) test-frontend-critical

test-frontend-critical:
	@cd frontend && pnpm exec vitest run $(FRONTEND_CRITICAL_VITEST)
