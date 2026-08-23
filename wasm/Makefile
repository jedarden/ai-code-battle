# Makefile for building all WASM artifacts per plan §11.2
# Builds: engine.wasm plus the thirteen sandbox opponent bots (plan §13.1):
#   original roster — gatherer, rusher, swarm, random, guardian, hunter
#   extended roster — farmer, opportunist, siege, economist (Go ladder bots)
#                     assassin, phalanx, zone-driver (Rust ladder bots)

.PHONY: all clean engine gatherer rusher swarm random guardian hunter \
        farmer opportunist siege economist assassin phalanx zone-driver

all: engine gatherer rusher swarm random guardian hunter \
     farmer opportunist siege economist assassin phalanx zone-driver
	@echo "All WASM artifacts built successfully"
	@ls -lh dist/

engine:
	@echo "Building engine.wasm..."
	cd engine && bash build.sh

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

farmer:
	@echo "Building farmer.wasm..."
	cd bots/farmer && ./build.sh

opportunist:
	@echo "Building opportunist.wasm..."
	cd bots/opportunist && ./build.sh

siege:
	@echo "Building siege.wasm..."
	cd bots/siege && ./build.sh

economist:
	@echo "Building economist.wasm..."
	cd bots/economist && ./build.sh

assassin:
	@echo "Building assassin.wasm..."
	cd bots/assassin && ./build.sh

phalanx:
	@echo "Building phalanx.wasm..."
	cd bots/phalanx && ./build.sh

zone-driver:
	@echo "Building zone-driver.wasm..."
	cd bots/zone-driver && ./build.sh

clean:
	@echo "Cleaning WASM build artifacts..."
	rm -rf dist/rusher dist/swarm
	rm -f dist/*.wasm
