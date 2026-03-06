# Makefile

# Nom du service (peut être redéfini via la ligne de commande)
NAME ?= mon_service

# Cible par défaut
all: up

.PHONY: up
up:
	@echo "Lancement de 'docker compose up --build' pour $(NAME)..."
	docker compose up --build

.PHONY: build
build:
	@echo "Build des conteneurs pour $(NAME)..."
	docker compose build

.PHONY: down
down:
	@echo "Arrêt des conteneurs pour $(NAME)..."
	docker compose down

.PHONY: logs
logs:
	@echo "Affichage des logs pour $(NAME)..."
	docker compose logs -f


.PHONY: rebuild
rebuild:
	@if [ -z "$(SERVICE)" ]; then \
		echo "Erreur : Il faut spécifier le service à reconstruire avec SERVICE=<nom_du_service>"; \
		echo "Exemple: make rebuild SERVICE=fastapi ou SERVICE=fastapi,front-marque"; \
		exit 1; \
	fi
	@SERVICES=$$(echo "$(SERVICE)" | tr ',' ' '); \
	echo "Reconstruction du conteneur '$$SERVICES'..."; \
	docker compose up --build $$SERVICES -d
