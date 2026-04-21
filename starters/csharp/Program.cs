// AI Code Battle - C# Starter Kit
//
// A minimal bot scaffold with HMAC authentication and a placeholder
// random strategy. Replace ComputeMoves() with your own logic.

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

app.MapPost("/turn", (HttpContext ctx) =>
{
    var signature = ctx.Request.Headers["X-ACB-Signature"].FirstOrDefault() ?? "";
    var matchId = ctx.Request.Headers["X-ACB-Match-Id"].FirstOrDefault() ?? "";
    var turnStr = ctx.Request.Headers["X-ACB-Turn"].FirstOrDefault() ?? "0";
    var timestamp = ctx.Request.Headers["X-ACB-Timestamp"].FirstOrDefault() ?? "";

    if (string.IsNullOrEmpty(signature))
        return Results.Unauthorized();

    using var reader = new StreamReader(ctx.Request.Body);
    var body = reader.ReadToEndAsync().GetAwaiter().GetResult();

    if (!VerifySignature(secret, matchId, turnStr, timestamp, body, signature))
        return Results.Unauthorized();

    GameState? state;
    try
    {
        state = JsonSerializer.Deserialize<GameState>(body);
        if (state == null) return Results.BadRequest("Invalid game state");
    }
    catch
    {
        return Results.BadRequest("Invalid JSON");
    }

    var moves = ComputeMoves(state);
    var responseBody = JsonSerializer.Serialize(new { moves });
    var turn = int.Parse(turnStr);
    var responseSig = SignResponse(secret, matchId, turn, responseBody);

    ctx.Response.Headers["X-ACB-Signature"] = responseSig;
    return Results.Text(responseBody, "application/json");
});

app.Run();

// --- Strategy ---
// Replace this with your own logic!

string[] Directions = ["N", "E", "S", "W"];

List<Move> ComputeMoves(GameState state)
{
    var moves = new List<Move>();
    var rng = Random.Shared;

    foreach (var bot in state.Bots)
    {
        if (bot.Owner == state.You.Id && rng.NextDouble() < 0.5)
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
    string timestamp, string body, string signature)
{
    var bodyHash = Sha256Hex(Encoding.UTF8.GetBytes(body));
    var signingString = $"{matchId}.{turn}.{timestamp}.{bodyHash}";
    var expected = HmacSha256(secret, signingString);
    return CryptographicOperations.FixedTimeEquals(
        Convert.FromHexString(signature),
        Convert.FromHexString(expected)
    );
}

static string SignResponse(string secret, string matchId, int turn, string body)
{
    var bodyHash = Sha256Hex(Encoding.UTF8.GetBytes(body));
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
    public string Direction { get; init; } = "";
}
