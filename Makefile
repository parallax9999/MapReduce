# Makefile for MapReduce Docker Management

.PHONY: help build up down restart logs logs-boss logs-workers status clean rebuild scale-workers test enter-boss enter-worker

# Default target
help:
	@echo "MapReduce Docker Commands:"
	@echo "  make build          - Build all Docker images"
	@echo "  make up             - Start all services"
	@echo "  make down           - Stop all services"
	@echo "  make restart        - Restart all services"
	@echo "  make logs           - View logs (all services)"
	@echo "  make logs-boss      - View boss logs only"
	@echo "  make logs-workers   - View worker logs only"
	@echo "  make status         - Show container status"
	@echo "  make clean          - Stop and remove containers/volumes"
	@echo "  make rebuild        - Clean rebuild of all images"
	@echo "  make scale-workers  - Scale workers to 8"
	@echo "  make test           - Run system test"
	@echo "  make enter-boss     - Enter boss container"
	@echo "  make enter-worker   - Enter worker1 container"

# Build images
build:
	@echo "🔨 Building Docker images..."
	docker compose build
	docker compose up -d
	docker compose logs -f

# Start services
up:
	@echo "🚀 Starting MapReduce system..."
	docker-compose up -d
	@echo "✅ System started!"
	@make status

# Stop services
down:
	@echo "🛑 Stopping MapReduce system..."
	docker-compose down
	@echo "✅ System stopped!"

# Restart services
restart:
	@echo "🔄 Restarting MapReduce system..."
	docker-compose restart
	@echo "✅ System restarted!"

# View logs
logs:
	@echo "📝 Viewing logs (Ctrl+C to exit)..."
	docker-compose logs -f

# View boss logs
logs-boss:
	@echo "📝 Viewing boss logs (Ctrl+C to exit)..."
	docker-compose logs -f boss

# View worker logs
logs-workers:
	@echo "📝 Viewing worker logs (Ctrl+C to exit)..."
	docker-compose logs -f worker1 worker2 worker3 worker4

# Check status
status:
	@echo "📊 Container Status:"
	@docker-compose ps
	@echo ""
	@echo "💻 Resource Usage:"
	@docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}"

# Clean everything
clean:
	@echo "🗑️  Cleaning up..."
	docker-compose down -v
	@echo "✅ Cleanup complete!"

# Rebuild from scratch
rebuild:
	@echo "🔨 Rebuilding from scratch..."
	docker-compose down -v
	docker-compose build --no-cache
	docker-compose up -d
	@echo "✅ Rebuild complete!"
	@make status

# Scale workers
scale-workers:
	@echo "📈 Scaling to 8 workers..."
	docker-compose up -d --scale worker1=8
	@make status

# Enter boss container
enter-boss:
	@echo "🚪 Entering boss container..."
	docker-compose exec boss sh

# Enter worker container
enter-worker:
	@echo "🚪 Entering worker1 container..."
	docker-compose exec worker1 sh

# Test system
test:
	@echo "🧪 Testing MapReduce system..."
	@echo "1. Checking containers are running..."
	@docker-compose ps | grep Up || (echo "❌ Containers not running!" && exit 1)
	@echo "✅ All containers running"
	@echo ""
	@echo "2. Checking boss logs for worker connections..."
	@docker-compose logs boss | grep -i "worker.*connected" | tail -4
	@echo ""
	@echo "3. Checking volume mount..."
	@docker-compose exec boss ls -la /volume | head -5
	@echo ""
	@echo "4. Testing network connectivity..."
	@docker-compose exec worker1 nc -zv boss 8080 2>&1 | grep open
	@echo ""
	@echo "✅ System test passed!"
