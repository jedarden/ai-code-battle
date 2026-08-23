.PHONY: all gatherer random guardian hunter rusher swarm \
        farmer opportunist siege economist assassin phalanx zone-driver clean

# Original six + extended roster (plan §13.1): the four Go ladder bots and the
# three Rust ladder bots, ported per bots/{farmer,opportunist,siege,economist}
# and bots/{assassin,phalanx,zone-driver}.
all: gatherer random guardian hunter rusher swarm \
     farmer opportunist siege economist assassin phalanx zone-driver

gatherer:
	cd gatherer && chmod +x build.sh && ./build.sh

random:
	cd random && chmod +x build.sh && ./build.sh

guardian:
	cd guardian && chmod +x build.sh && ./build.sh

hunter:
	cd hunter && chmod +x build.sh && ./build.sh

rusher:
	cd rusher && chmod +x build.sh && ./build.sh

swarm:
	cd swarm && chmod +x build.sh && ./build.sh

farmer:
	cd farmer && chmod +x build.sh && ./build.sh

opportunist:
	cd opportunist && chmod +x build.sh && ./build.sh

siege:
	cd siege && chmod +x build.sh && ./build.sh

economist:
	cd economist && chmod +x build.sh && ./build.sh

assassin:
	cd assassin && chmod +x build.sh && ./build.sh

phalanx:
	cd phalanx && chmod +x build.sh && ./build.sh

zone-driver:
	cd zone-driver && chmod +x build.sh && ./build.sh

clean:
	rm -rf dist
	mkdir -p dist
