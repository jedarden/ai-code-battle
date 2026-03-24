<?php
/**
 * GuardianBot - A defensive bot that protects cores and gathers nearby energy.
 *
 * Strategy: Defend own core, gather nearby energy, cautious expansion.
 * - Maintain a perimeter of bots within 5 tiles of each owned core
 * - Assign excess bots to gather energy within 10 tiles of a core
 * - Consolidate defenders when enemies approach
 * - Only send scouts to explore beyond the safe zone
 * - Conservative spawning - maintains energy reserve of 6
 */

require_once __DIR__ . '/game.php';
require_once __DIR__ . '/strategy.php';

// Get configuration from environment
$port = getenv('BOT_PORT') ?: '8083';
$secret = getenv('BOT_SECRET');

if (!$secret) {
    fwrite(STDERR, "ERROR: BOT_SECRET environment variable is required\n");
    exit(1);
}

$strategy = new GuardianStrategy();

// Build HTTP server using PHP built-in
$server = stream_socket_server("tcp://0.0.0.0:$port", $errno, $errstr);
if (!$server) {
    fwrite(STDERR, "Failed to create server: $errstr ($errno)\n");
    exit(1);
}

fwrite(STDOUT, "GuardianBot starting on port $port\n");

while ($conn = stream_socket_accept($server)) {
    handle_request($conn, $secret, $strategy);
    fclose($conn);
}

/**
 * Handle an incoming HTTP request
 */
function handle_request($conn, string $secret, GuardianStrategy $strategy): void {
    // Read request
    $request = fread($conn, 65536);

    // Parse request line
    $lines = explode("\r\n", $request);
    $requestLine = explode(' ', $lines[0] ?? '');
    $method = $requestLine[0] ?? '';
    $path = $requestLine[1] ?? '/';

    // Parse headers
    $headers = [];
    $bodyStart = 0;
    for ($i = 1; $i < count($lines); $i++) {
        if ($lines[$i] === '') {
            $bodyStart = $i + 1;
            break;
        }
        $parts = explode(': ', $lines[$i], 2);
        if (count($parts) === 2) {
            $headers[$parts[0]] = $parts[1];
        }
    }

    // Extract body
    $body = implode("\r\n", array_slice($lines, $bodyStart));

    // Route request
    if ($method === 'GET' && $path === '/health') {
        send_response($conn, 200, 'text/plain', 'OK');
        return;
    }

    if ($method === 'POST' && $path === '/turn') {
        handle_turn($conn, $secret, $strategy, $headers, $body);
        return;
    }

    send_response($conn, 404, 'text/plain', 'Not Found');
}

/**
 * Handle turn request
 */
function handle_turn($conn, string $secret, GuardianStrategy $strategy, array $headers, string $body): void {
    // Extract auth headers
    $matchId = $headers['X-ACB-Match-Id'] ?? '';
    $turnStr = $headers['X-ACB-Turn'] ?? '';
    $timestamp = $headers['X-ACB-Timestamp'] ?? '';
    $signature = $headers['X-ACB-Signature'] ?? '';

    if (!$matchId || !$turnStr || !$timestamp || !$signature) {
        send_response($conn, 401, 'text/plain', 'Missing auth headers');
        return;
    }

    // Verify signature
    if (!verify_signature($secret, $matchId, $turnStr, $timestamp, $body, $signature)) {
        send_response($conn, 401, 'text/plain', 'Invalid signature');
        return;
    }

    // Parse game state
    $state = json_decode($body, true);
    if (!$state) {
        send_response($conn, 400, 'text/plain', 'Invalid JSON');
        return;
    }

    $gameState = GameState::fromArray($state);

    // Compute moves
    $moves = $strategy->computeMoves($gameState);

    // Build response
    $response = ['moves' => array_map(fn($m) => $m->toArray(), $moves)];
    $responseBody = json_encode($response);

    // Sign response
    $turn = (int)$turnStr;
    $responseSig = sign_response($secret, $matchId, $turn, $responseBody);

    $headers = [
        'Content-Type: application/json',
        "X-ACB-Signature: $responseSig"
    ];

    send_response($conn, 200, 'application/json', $responseBody, $headers);
}

/**
 * Verify HMAC signature
 */
function verify_signature(string $secret, string $matchId, string $turn, string $timestamp, string $body, string $signature): bool {
    $bodyHash = hash('sha256', $body);
    $signingString = "$matchId.$turn.$timestamp.$bodyHash";
    $expected = hash_hmac('sha256', $signingString, $secret);
    return hash_equals($expected, $signature);
}

/**
 * Sign response body
 */
function sign_response(string $secret, string $matchId, int $turn, string $body): string {
    $bodyHash = hash('sha256', $body);
    $signingString = "$matchId.$turn.$bodyHash";
    return hash_hmac('sha256', $signingString, $secret);
}

/**
 * Send HTTP response
 */
function send_response($conn, int $status, string $contentType, string $body, array $extraHeaders = []): void {
    $statusText = [
        200 => 'OK',
        400 => 'Bad Request',
        401 => 'Unauthorized',
        404 => 'Not Found',
    ][$status] ?? 'Unknown';

    $response = "HTTP/1.1 $status $statusText\r\n";
    $response .= "Content-Type: $contentType\r\n";
    $response .= "Content-Length: " . strlen($body) . "\r\n";
    foreach ($extraHeaders as $header) {
        $response .= "$header\r\n";
    }
    $response .= "\r\n";
    $response .= $body;

    fwrite($conn, $response);
}
