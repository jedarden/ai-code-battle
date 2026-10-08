using System;
using System.Collections.Generic;
using System.Linq;
using System.Text.Json;

static class ProtocolSchema
{
    private static readonly string[] RequiredTop = ["match_id", "turn", "config", "you", "bots", "energy", "cores", "walls", "dead"];
    private static readonly string[] RequiredConfig = ["rows", "cols", "max_turns", "vision_radius2", "attack_radius2", "spawn_cost", "energy_interval", "cores_per_player", "zone_enabled", "zone_start_turn", "zone_shrink_interval", "zone_shrink_step", "zone_min_radius", "kill_score"];
    private static readonly string[] OptionalConfig = ["map_id", "season_id", "rules_version", "turn_timeout"];

    public static bool IsValidRequest(JsonElement state)
    {
        if (state.ValueKind != JsonValueKind.Object ||
            !HasOnlyFields(state, RequiredTop.Concat(["zone"])) ||
            !HasRequiredFields(state, RequiredTop) ||
            !IsNonEmptyString(state, "match_id") || !IsInt32(state, "turn"))
            return false;

        var config = state.GetProperty("config");
        if (config.ValueKind != JsonValueKind.Object ||
            !HasOnlyFields(config, RequiredConfig.Concat(OptionalConfig)) ||
            !HasRequiredFields(config, RequiredConfig))
            return false;
        foreach (var name in RequiredConfig)
        {
            if (name == "zone_enabled" ? !IsBoolean(config, name) : !IsInt32(config, name))
                return false;
        }
        foreach (var name in new[] { "map_id", "season_id", "rules_version" })
        {
            if (config.TryGetProperty(name, out var value) && value.ValueKind != JsonValueKind.String)
                return false;
        }
        if (config.TryGetProperty("turn_timeout", out var timeout) &&
            (timeout.ValueKind != JsonValueKind.Number || !timeout.TryGetInt64(out _)))
            return false;

        var you = state.GetProperty("you");
        if (you.ValueKind != JsonValueKind.Object ||
            !HasOnlyFields(you, ["id", "energy", "score"]) ||
            !HasRequiredFields(you, ["id", "energy", "score"]) ||
            !IsInt32(you, "id") || !IsInt32(you, "energy") || !IsInt32(you, "score"))
            return false;

        if (!IsArrayOf(state, "bots", IsBot) || !IsArrayOf(state, "dead", IsBot) ||
            !IsArrayOf(state, "energy", IsPosition) || !IsArrayOf(state, "walls", IsPosition) ||
            !IsArrayOf(state, "cores", IsCore))
            return false;

        if (state.TryGetProperty("zone", out var zone) && zone.ValueKind != JsonValueKind.Null &&
            (zone.ValueKind != JsonValueKind.Object ||
             !HasOnlyFields(zone, ["center", "radius", "active"]) ||
             !HasRequiredFields(zone, ["center", "radius", "active"]) ||
             !IsPosition(zone.GetProperty("center")) || !IsInt32(zone, "radius") ||
             !IsBoolean(zone, "active")))
            return false;

        return true;
    }

    private static bool IsArrayOf(JsonElement obj, string name, Func<JsonElement, bool> validate)
    {
        if (!obj.TryGetProperty(name, out var array) || array.ValueKind != JsonValueKind.Array)
            return false;
        return array.EnumerateArray().All(validate);
    }

    private static bool IsPosition(JsonElement value) =>
        value.ValueKind == JsonValueKind.Object &&
        HasOnlyFields(value, ["row", "col"]) && HasRequiredFields(value, ["row", "col"]) &&
        IsInt32(value, "row") && IsInt32(value, "col");

    private static bool IsBot(JsonElement value) =>
        value.ValueKind == JsonValueKind.Object &&
        HasOnlyFields(value, ["position", "owner"]) && HasRequiredFields(value, ["position", "owner"]) &&
        IsPosition(value.GetProperty("position")) && IsInt32(value, "owner");

    private static bool IsCore(JsonElement value) =>
        value.ValueKind == JsonValueKind.Object &&
        HasOnlyFields(value, ["position", "owner", "active"]) && HasRequiredFields(value, ["position", "owner", "active"]) &&
        IsPosition(value.GetProperty("position")) && IsInt32(value, "owner") && IsBoolean(value, "active");

    private static bool IsNonEmptyString(JsonElement obj, string name) =>
        obj.TryGetProperty(name, out var value) && value.ValueKind == JsonValueKind.String && !string.IsNullOrEmpty(value.GetString());

    private static bool IsInt32(JsonElement obj, string name) =>
        obj.TryGetProperty(name, out var value) && value.ValueKind == JsonValueKind.Number && value.TryGetInt32(out _);

    private static bool IsBoolean(JsonElement obj, string name) =>
        obj.TryGetProperty(name, out var value) && (value.ValueKind == JsonValueKind.True || value.ValueKind == JsonValueKind.False);

    private static bool HasRequiredFields(JsonElement obj, IEnumerable<string> required) =>
        required.All(name => obj.TryGetProperty(name, out _));

    private static bool HasOnlyFields(JsonElement obj, IEnumerable<string> allowed)
    {
        var allowedNames = new HashSet<string>(allowed, StringComparer.Ordinal);
        return obj.EnumerateObject().All(property => allowedNames.Contains(property.Name));
    }
}
