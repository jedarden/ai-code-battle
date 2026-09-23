// AI Code Battle - C# Starter Kit
//
// A minimal bot scaffold with HMAC authentication and a placeholder
// random strategy. Replace ComputeMoves() with your own logic.

using System.Globalization;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.DependencyInjection;

var port = Environment.GetEnvironmentVariable("BOT_PORT") ?? "8080";
var secret = Environment.GetEnvironmentVariable("BOT_SECRET") ?? "";

if (string.IsNullOrEmpty(secret))
{
    Console.Error.WriteLine("ERROR: BOT_SECRET environment variable is required");
    Environment.Exit(1);
}

var builder = WebApplication.CreateBuilder(args);
builder.WebHost.UseUrls($"http://0.0.0.0:{port}");
var app = builder.Build();

app.MapGet("/health", () => Results.Ok("OK"));

// --- Constants ---
string[] Directions = ["N", "E", "S", "W"];

var jsonOptions = new JsonSerializerOptions
{
    PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower
};

app.MapPost("/turn", (HttpContext ctx) =>
{
    var signature = ctx.Request.Headers["X-ACB-Signature"].FirstOrDefault() ?? "";
    var matchId = ctx.Request.Headers["X-ACB-Match-Id"].FirstOrDefault() ?? "";
    var turnStr = ctx.Request.Headers["X-ACB-Turn"].FirstOrDefault() ?? "";
    var timestamp = ctx.Request.Headers["X-ACB-Timestamp"].FirstOrDefault() ?? "";
    var botId = ctx.Request.Headers["X-ACB-Bot-Id"].FirstOrDefault() ?? "";

    if (string.IsNullOrEmpty(matchId) || string.IsNullOrEmpty(turnStr) ||
        string.IsNullOrEmpty(timestamp) || string.IsNullOrEmpty(botId) ||
        string.IsNullOrEmpty(signature))
        return Results.Unauthorized();

    if (!int.TryParse(turnStr, NumberStyles.None, CultureInfo.InvariantCulture,
            out var turn) || turn < 0)
        return Results.Unauthorized();

    using var bodyStream = new MemoryStream();
    ctx.Request.Body.CopyToAsync(bodyStream).GetAwaiter().GetResult();
    var body = bodyStream.ToArray();

    if (!VerifySignature(secret, matchId, turnStr, timestamp, body, signature))
        return Results.Unauthorized();

    if (!VerifyTimestamp(timestamp))
        return Results.Unauthorized();

    GameState? state;
    try
    {
        using var document = JsonDocument.Parse(body);
        if (document.RootElement.ValueKind != JsonValueKind.Object ||
            !document.RootElement.TryGetProperty("match_id", out var bodyMatchId) ||
            bodyMatchId.ValueKind != JsonValueKind.String ||
            bodyMatchId.GetString() != matchId ||
            !document.RootElement.TryGetProperty("turn", out var bodyTurn) ||
            bodyTurn.ValueKind != JsonValueKind.Number ||
            !bodyTurn.TryGetInt32(out var bodyTurnNumber) ||
            bodyTurnNumber != turn)
            return Results.Unauthorized();

        state = JsonSerializer.Deserialize<GameState>(body, jsonOptions);
        if (state == null) return Results.BadRequest("Invalid game state");
    }
    catch (JsonException)
    {
        return Results.BadRequest("Invalid JSON");
    }

    if (state.Turn == 0)
    {
        var seasonId = state.Config.SeasonId ?? "";
        var rulesVersion = state.Config.RulesVersion ?? "";
        Console.WriteLine($"match={state.MatchId} season_id={seasonId} rules_version={rulesVersion} rows={state.Config.Rows} cols={state.Config.Cols}");
    }

    var moves = ComputeMoves(state);
    var responseBody = JsonSerializer.SerializeToUtf8Bytes(new { moves }, jsonOptions);
    var responseSig = SignResponse(secret, matchId, turn, responseBody);

    ctx.Response.Headers["X-ACB-Signature"] = responseSig;
    return Results.Bytes(responseBody, "application/json");
});

app.Run();

// --- Strategy ---
// Replace this with your own logic!

List<Move> ComputeMoves(GameState state)
{
    var rows = state.Config.Rows;
    var cols = state.Config.Cols;
    var moves = new List<Move>();
    var rng = Random.Shared;

    var cardinal = new (int dr, int dc, string dir)[] {
        (-1, 0, "N"), (0, 1, "E"), (1, 0, "S"), (0, -1, "W"),
    };

    foreach (var bot in state.Bots)
    {
        if (bot.Owner != state.You.Id) continue;

        // Find direction toward nearest energy using toroidal distance
        if (state.Energy.Count > 0)
        {
            int bestDist = int.MaxValue;
            string? bestDir = null;
            foreach (var (dr, dc, dir) in cardinal)
            {
                var nr = (bot.Position.Row + dr + rows) % rows;
                var nc = (bot.Position.Col + dc + cols) % cols;
                foreach (var e in state.Energy)
                {
                    var d = Grid.ToroidalManhattan(nr, nc, e.Row, e.Col, rows, cols);
                    if (d < bestDist)
                    {
                        bestDist = d;
                        bestDir = dir;
                    }
                }
            }
            if (bestDir != null)
            {
                moves.Add(new Move
                {
                    Position = bot.Position,
                    Direction = bestDir
                });
                continue;
            }
        }

        if (rng.NextDouble() < 0.5)
        {
            moves.Add(new Move
            {
                Position = bot.Position,
                Direction = Directions[rng.Next(Directions.Length)]
            });
        }
    }

    return moves;
}

// --- HMAC helpers ---

static bool VerifySignature(string secret, string matchId, string turn,
    string timestamp, byte[] body, string signature)
{
    var bodyHash = Sha256Hex(body);
    var signingString = $"{matchId}.{turn}.{timestamp}.{bodyHash}";
    var expected = HmacSha256(secret, signingString);
    if (signature.Length != expected.Length)
        return false;

    try
    {
        return CryptographicOperations.FixedTimeEquals(
            Convert.FromHexString(signature),
            Convert.FromHexString(expected)
        );
    }
    catch (FormatException)
    {
        return false;
    }
}

static bool VerifyTimestamp(string timestamp)
{
    if (!long.TryParse(timestamp, NumberStyles.None, CultureInfo.InvariantCulture,
            out var seconds))
        return false;

    var now = DateTimeOffset.UtcNow.ToUnixTimeSeconds();
    return seconds >= now - 30 && seconds <= now + 30;
}

static string SignResponse(string secret, string matchId, int turn, byte[] body)
{
    // Response signing string: {match_id}.{turn}.{sha256_hex(body)}
    var bodyHash = Sha256Hex(body);
    var signingString = $"{matchId}.{turn}.{bodyHash}";
    return HmacSha256(secret, signingString);
}

static string HmacSha256(string key, string data)
{
    using var hmac = new HMACSHA256(Encoding.UTF8.GetBytes(key));
    return Convert.ToHexString(hmac.ComputeHash(Encoding.UTF8.GetBytes(data))).ToLower();
}

static string Sha256Hex(byte[] data)
{
    return Convert.ToHexString(SHA256.HashData(data)).ToLower();
}

// --- Types ---

record GameState
{
    public string MatchId { get; init; } = "";
    public int Turn { get; init; }
    public GameConfig Config { get; init; } = new();
    public You You { get; init; } = new();
    public List<VisibleBot> Bots { get; init; } = [];
    public List<Position> Energy { get; init; } = [];
    public List<VisibleCore> Cores { get; init; } = [];
    public List<Position> Walls { get; init; } = [];
    public List<VisibleBot> Dead { get; init; } = [];
}

record GameConfig
{
    public int Rows { get; init; }
    public int Cols { get; init; }
    public int MaxTurns { get; init; }
    public int VisionRadius2 { get; init; }
    public int AttackRadius2 { get; init; }
    public int SpawnCost { get; init; }
    public int EnergyInterval { get; init; }
    public string? SeasonId { get; init; }
    public string? RulesVersion { get; init; }
}

record You
{
    public int Id { get; init; }
    public int Energy { get; init; }
    public int Score { get; init; }
}

record VisibleBot
{
    public Position Position { get; init; } = new();
    public int Owner { get; init; }
}

record VisibleCore
{
    public Position Position { get; init; } = new();
    public int Owner { get; init; }
    public bool Active { get; init; }
}

record Position
{
    public int Row { get; init; }
    public int Col { get; init; }
}

record Move
{
    public Position Position { get; init; } = new();
    public string Direction { get; init; } = "stay";
}
