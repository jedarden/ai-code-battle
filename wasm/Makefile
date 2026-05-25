# Makefile for building all WASM bots per plan §11.2
# Builds: gatherer.wasm, rusher.wasm, swarm.wasm, random.wasm, guardian.wasm, hunter.wasm

.PHONY: all clean gatherer rusher swarm random guardian hunter

all: gatherer rusher swarm random guardian hunter
	@echo "All WASM bots built successfully"
	@ls -lh dist/

gatherer:
	@echo "Building gatherer.wasm..."
	cd bots/gatherer && ./build.sh

rusher:
	@echo "Building rusher.wasm..."
	cd bots/rusher && wasm-pack build --target web --out-dir ../../dist/rusher
	cp dist/rusher/rusher_wasm_bg.wasm dist/rusher.wasm 2>/dev/null || true
	# Try alternate output paths
	@if [ -f dist/rusher/rusher_wasm_bg.wasm ]; then \
		cp dist/rusher/rusher_wasm_bg.wasm dist/rusher.wasm; \
	elif [ -f dist/rusher/rusher_wasm.wasm ]; then \
		cp dist/rusher/rusher_wasm.wasm dist/rusher.wasm; \
	elif [ -f target/wasm32-unknown-unknown/release/rusher_wasm.wasm ]; then \
		cp target/wasm32-unknown-unknown/release/rusher_wasm.wasm dist/rusher.wasm; \
	fi

swarm:
	@echo "Building swarm.wasm..."
	cd bots/swarm && ./build.sh

random:
	@echo "Building random.wasm..."
	cd bots/random && ./build.sh

guardian:
	@echo "Building guardian.wasm..."
	cd bots/guardian && ./build.sh

hunter:
	@echo "Building hunter.wasm..."
	cd bots/hunter && ./build.sh

clean:
	@echo "Cleaning WASM build artifacts..."
	rm -rf dist/rusher dist/swarm
	rm -f dist/*.wasm
