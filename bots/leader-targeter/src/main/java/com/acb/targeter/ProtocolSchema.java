package com.acb.targeter;

import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.util.Iterator;
import java.util.Set;
import java.util.function.Predicate;

final class ProtocolSchema {
    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final Set<String> TOP = Set.of("match_id", "turn", "config", "you", "bots", "energy", "cores", "walls", "dead", "zone");
    private static final Set<String> TOP_REQUIRED = Set.of("match_id", "turn", "config", "you", "bots", "energy", "cores", "walls", "dead");
    private static final Set<String> CONFIG_REQUIRED = Set.of("rows", "cols", "max_turns", "vision_radius2", "attack_radius2", "spawn_cost", "energy_interval", "cores_per_player", "zone_enabled", "zone_start_turn", "zone_shrink_interval", "zone_shrink_step", "zone_min_radius", "kill_score");
    private static final Set<String> CONFIG = Set.of("rows", "cols", "max_turns", "vision_radius2", "attack_radius2", "spawn_cost", "energy_interval", "cores_per_player", "zone_enabled", "zone_start_turn", "zone_shrink_interval", "zone_shrink_step", "zone_min_radius", "kill_score", "map_id", "season_id", "rules_version", "turn_timeout");

    private ProtocolSchema() {}

    static JsonNode parse(byte[] body) throws IOException {
        try (JsonParser parser = MAPPER.getFactory().createParser(body)) {
            JsonNode root = MAPPER.readTree(parser);
            if (root == null || parser.nextToken() != null) throw new IOException("expected exactly one JSON value");
            return root;
        }
    }

    static boolean isValid(JsonNode root) {
        if (!root.isObject() || !only(root, TOP) || !required(root, TOP_REQUIRED) ||
                !nonEmptyString(root, "match_id") || !integer(root, "turn")) return false;
        JsonNode config = root.get("config");
        if (!config.isObject() || !only(config, CONFIG) || !required(config, CONFIG_REQUIRED)) return false;
        for (String key : CONFIG_REQUIRED) {
            if (key.equals("zone_enabled") ? !config.get(key).isBoolean() : !integer(config, key)) return false;
        }
        for (String key : Set.of("map_id", "season_id", "rules_version")) {
            if (config.has(key) && !config.get(key).isTextual()) return false;
        }
        if (config.has("turn_timeout") && (!config.get("turn_timeout").isIntegralNumber() ||
                !config.get("turn_timeout").canConvertToLong())) return false;

        JsonNode you = root.get("you");
        if (!you.isObject() || !only(you, Set.of("id", "energy", "score")) ||
                !required(you, Set.of("id", "energy", "score")) ||
                !integer(you, "id") || !integer(you, "energy") || !integer(you, "score")) return false;

        if (!arrayOf(root, "bots", ProtocolSchema::bot) || !arrayOf(root, "dead", ProtocolSchema::bot) ||
                !arrayOf(root, "energy", ProtocolSchema::position) || !arrayOf(root, "walls", ProtocolSchema::position) ||
                !arrayOf(root, "cores", ProtocolSchema::core)) return false;

        if (root.has("zone") && !root.get("zone").isNull()) {
            JsonNode zone = root.get("zone");
            if (!zone.isObject() || !only(zone, Set.of("center", "radius", "active")) ||
                    !required(zone, Set.of("center", "radius", "active")) ||
                    !position(zone.get("center")) || !integer(zone, "radius") || !zone.get("active").isBoolean()) return false;
        }
        return true;
    }

    private static boolean arrayOf(JsonNode object, String key, Predicate<JsonNode> check) {
        JsonNode array = object.get(key);
        if (array == null || !array.isArray()) return false;
        for (JsonNode item : array) if (!check.test(item)) return false;
        return true;
    }

    private static boolean position(JsonNode node) {
        return node.isObject() && only(node, Set.of("row", "col")) && required(node, Set.of("row", "col")) && integer(node, "row") && integer(node, "col");
    }

    private static boolean bot(JsonNode node) {
        return node.isObject() && only(node, Set.of("position", "owner")) && required(node, Set.of("position", "owner")) && position(node.get("position")) && integer(node, "owner");
    }

    private static boolean core(JsonNode node) {
        return node.isObject() && only(node, Set.of("position", "owner", "active")) && required(node, Set.of("position", "owner", "active")) && position(node.get("position")) && integer(node, "owner") && node.get("active").isBoolean();
    }

    private static boolean nonEmptyString(JsonNode node, String key) {
        return node.has(key) && node.get(key).isTextual() && !node.get(key).asText().isEmpty();
    }

    private static boolean integer(JsonNode node, String key) {
        return node.has(key) && node.get(key).isIntegralNumber() && node.get(key).canConvertToInt();
    }

    private static boolean required(JsonNode node, Set<String> fields) {
        for (String field : fields) if (!node.has(field)) return false;
        return true;
    }

    private static boolean only(JsonNode node, Set<String> fields) {
        Iterator<String> names = node.fieldNames();
        while (names.hasNext()) if (!fields.contains(names.next())) return false;
        return true;
    }
}
