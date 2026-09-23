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
    $request = '';
    while (($headerEnd = strpos($request, "\r\n\r\n")) === false) {
        $byte = fread($conn, 1);
        if ($byte === false || $byte === '') {
            send_response($conn, 400, 'text/plain', 'Invalid request');
            return;
        }
        $request .= $byte;
        if (strlen($request) > 65536) {
            send_response($conn, 400, 'text/plain', 'Invalid request');
            return;
        }
    }

    $lines = explode("\r\n", substr($request, 0, $headerEnd));
    $requestLine = explode(' ', $lines[0] ?? '');
    $method = $requestLine[0] ?? '';
    $path = $requestLine[1] ?? '/';

    $headers = [];
    for ($i = 1; $i < count($lines); $i++) {
        $parts = explode(':', $lines[$i], 2);
        if (count($parts) === 2) {
            $name = strtolower(trim($parts[0]));
            $headers[$name] = array_key_exists($name, $headers) ? null : trim($parts[1]);
        }
    }

    $contentLengthHeader = get_header($headers, 'Content-Length');
    $contentLength = 0;
    if ($contentLengthHeader !== '') {
        if (!ctype_digit($contentLengthHeader)) {
            send_response($conn, 400, 'text/plain', 'Invalid Content-Length');
            return;
        }
        $contentLength = filter_var($contentLengthHeader, FILTER_VALIDATE_INT, [
            'options' => ['min_range' => 0],
        ]);
        if ($contentLength === false) {
            send_response($conn, 400, 'text/plain', 'Invalid Content-Length');
            return;
        }
    }

    $body = '';
    while (strlen($body) < $contentLength) {
        $remaining = $contentLength - strlen($body);
        $chunk = fread($conn, min(8192, $remaining));
        if ($chunk === false || $chunk === '') {
            send_response($conn, 400, 'text/plain', 'Invalid request body');
            return;
        }
        $body .= $chunk;
    }

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

function get_header(array $headers, string $name): string {
    foreach ($headers as $headerName => $value) {
        if (strcasecmp($headerName, $name) === 0) {
            return is_string($value) ? $value : '';
        }
    }
    return '';
}

/**
 * Handle turn request
 */
function handle_turn($conn, string $secret, GuardianStrategy $strategy, array $headers, string $body): void {
    // Extract auth headers
    $matchId = get_header($headers, 'X-ACB-Match-Id');
    $turnStr = get_header($headers, 'X-ACB-Turn');
    $timestamp = get_header($headers, 'X-ACB-Timestamp');
    $botId = get_header($headers, 'X-ACB-Bot-Id');
    $signature = get_header($headers, 'X-ACB-Signature');
    $turn = ctype_digit($turnStr)
        ? filter_var($turnStr, FILTER_VALIDATE_INT, ['options' => ['min_range' => 0]])
        : false;

    if ($matchId === '' || $turn === false || $timestamp === '' || $botId === '' || $signature === '') {
        send_response($conn, 401, 'text/plain', 'Missing auth headers');
        return;
    }

    // Verify signature
    if (!verify_signature($secret, $matchId, $turnStr, $timestamp, $body, $signature)) {
        send_response($conn, 401, 'text/plain', 'Invalid signature');
        return;
    }

    if (!verify_timestamp($timestamp)) {
        send_response($conn, 401, 'text/plain', 'Invalid timestamp');
        return;
    }

    // Parse game state
    $state = json_decode($body, true);
    if (json_last_error() !== JSON_ERROR_NONE) {
        send_response($conn, 400, 'text/plain', 'Invalid JSON');
        return;
    }

    if (!is_array($state) ||
        !array_key_exists('match_id', $state) ||
        !is_string($state['match_id']) ||
        !array_key_exists('turn', $state) ||
        !is_int($state['turn']) ||
        $state['match_id'] !== $matchId ||
        $state['turn'] !== $turn
    ) {
        send_response($conn, 401, 'text/plain', 'Invalid request identity');
        return;
    }

    $gameState = GameState::fromArray($state);

    // Compute moves
    $moves = $strategy->computeMoves($gameState);

    // Build response
    $response = ['moves' => array_map(fn($m) => $m->toArray(), $moves)];
    $responseBody = json_encode($response);

    // Sign response
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
    if (!preg_match('/\A[0-9a-fA-F]{64}\z/', $signature)) {
        return false;
    }
    $bodyHash = hash('sha256', $body);
    $signingString = "$matchId.$turn.$timestamp.$bodyHash";
    $expected = hash_hmac('sha256', $signingString, $secret);
    return hash_equals($expected, $signature);
}

function verify_timestamp(string $timestamp): bool {
    if (!ctype_digit($timestamp)) {
        return false;
    }
    $seconds = filter_var($timestamp, FILTER_VALIDATE_INT, [
        'options' => ['min_range' => 0],
    ]);
    return $seconds !== false && abs(time() - $seconds) <= 30;
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
