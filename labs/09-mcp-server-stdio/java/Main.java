import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.io.*;
import java.time.ZoneId;
import java.time.ZonedDateTime;
import java.util.List;

public class Main {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private static final List<JsonNode> TOOLS = List.of(
            MAPPER.createObjectNode()
                    .put("name", "get_current_time")
                    .put("description", "Return the current time in ISO 8601 format.")
                    .set("inputSchema", MAPPER.createObjectNode()
                            .put("type", "object")
                            .set("properties", MAPPER.createObjectNode()
                                    .set("timezone", MAPPER.createObjectNode()
                                            .put("type", "string")
                                            .put("description", "IANA timezone name, e.g. UTC or Asia/Shanghai.")))),
            MAPPER.createObjectNode()
                    .put("name", "calculate")
                    .put("description", "Evaluate a simple arithmetic expression.")
                    .set("inputSchema", MAPPER.createObjectNode()
                            .put("type", "object")
                            .set("properties", MAPPER.createObjectNode()
                                    .set("expression", MAPPER.createObjectNode()
                                            .put("type", "string")
                                            .put("description", "Arithmetic expression with +, -, *, /, parentheses, numbers.")))
                            .set("required", MAPPER.createArrayNode().add("expression")))
    );

    public static void main(String[] args) throws Exception {
        if (args.length > 0 && "--smoke".equals(args[0])) {
            smokeTest();
        } else {
            serve(System.in, System.out);
        }
    }

    private static void serve(InputStream in, OutputStream out) throws Exception {
        System.err.println("MCP server ready on stdio");
        while (true) {
            JsonNode request;
            try {
                request = readMessage(in);
            } catch (Exception exc) {
                System.err.println("Bad JSON: " + exc.getMessage());
                writeMessage(out, makeError(null, -32700, "Parse error"));
                continue;
            }
            if (request == null) {
                System.err.println("EOF reached; shutting down");
                break;
            }
            JsonNode response = handleRequest(request);
            if (response != null) {
                writeMessage(out, response);
            }
        }
    }

    private static JsonNode readMessage(InputStream in) throws Exception {
        int contentLength = 0;
        while (true) {
            String line = readLine(in);
            if (line == null) {
                return null;
            }
            line = line.trim();
            if (line.isEmpty()) {
                break;
            }
            if (line.contains(":")) {
                String[] parts = line.split(":", 2);
                if ("content-length".equalsIgnoreCase(parts[0].trim())) {
                    contentLength = Integer.parseInt(parts[1].trim());
                }
            }
        }
        if (contentLength == 0) {
            return null;
        }
        byte[] body = new byte[contentLength];
        int read = 0;
        while (read < contentLength) {
            int n = in.read(body, read, contentLength - read);
            if (n < 0) {
                return null;
            }
            read += n;
        }
        return MAPPER.readTree(new String(body, java.nio.charset.StandardCharsets.UTF_8));
    }

    private static String readLine(InputStream in) throws IOException {
        ByteArrayOutputStream buf = new ByteArrayOutputStream();
        int b;
        while ((b = in.read()) != -1) {
            if (b == '\n') {
                break;
            }
            if (b != '\r') {
                buf.write(b);
            }
        }
        if (b == -1 && buf.size() == 0) {
            return null;
        }
        return buf.toString(java.nio.charset.StandardCharsets.UTF_8);
    }

    private static void writeMessage(OutputStream out, JsonNode message) throws Exception {
        byte[] body = MAPPER.writeValueAsBytes(message);
        out.write(("Content-Length: " + body.length + "\r\n\r\n").getBytes(java.nio.charset.StandardCharsets.UTF_8));
        out.write(body);
        out.flush();
    }

    private static JsonNode handleRequest(JsonNode request) {
        String method = request.get("method").asText();
        JsonNode params = request.get("params");
        if (params == null) {
            params = MAPPER.createObjectNode();
        }
        JsonNode reqId = request.get("id");

        System.err.println("Received " + method);

        return switch (method) {
            case "initialize" -> makeResponse(reqId, MAPPER.createObjectNode()
                    .put("protocolVersion", "2024-11-05")
                    .set("capabilities", MAPPER.createObjectNode().set("tools", MAPPER.createObjectNode()))
                    .set("serverInfo", MAPPER.createObjectNode().put("name", "agent-labs-mcp-server").put("version", "0.1.0")));
            case "notifications/initialized" -> null;
            case "tools/list" -> makeResponse(reqId, MAPPER.createObjectNode().set("tools", MAPPER.valueToTree(TOOLS)));
            case "tools/call" -> {
                String name = params.get("name").asText();
                JsonNode arguments = params.get("arguments");
                yield switch (name) {
                    case "get_current_time" -> makeResponse(reqId, getCurrentTime(arguments));
                    case "calculate" -> makeResponse(reqId, calculate(arguments));
                    default -> makeError(reqId, -32601, "Unknown tool: " + name);
                };
            }
            default -> makeError(reqId, -32601, "Method not found: " + method);
        };
    }

    private static JsonNode makeResponse(JsonNode id, JsonNode result) {
        ObjectNode response = MAPPER.createObjectNode();
        response.put("jsonrpc", "2.0");
        response.set("id", id);
        response.set("result", result);
        return response;
    }

    private static JsonNode makeError(JsonNode id, int code, String message) {
        ObjectNode response = MAPPER.createObjectNode();
        response.put("jsonrpc", "2.0");
        response.set("id", id);
        ObjectNode error = response.putObject("error");
        error.put("code", code);
        error.put("message", message);
        return response;
    }

    private static JsonNode getCurrentTime(JsonNode arguments) {
        String timezone = arguments.has("timezone") ? arguments.get("timezone").asText() : "UTC";
        try {
            ZoneId zone = ZoneId.of(timezone);
            String iso = ZonedDateTime.now(zone).toOffsetDateTime().toString();
            return resultNode(iso, false);
        } catch (Exception exc) {
            if (timezone.equalsIgnoreCase("UTC")) {
                return resultNode(ZonedDateTime.now(ZoneId.of("UTC")).toOffsetDateTime().toString(), false);
            }
            return resultNode("Unsupported timezone: " + timezone + ". Using UTC.", true);
        }
    }

    private static JsonNode calculate(JsonNode arguments) {
        String expression = arguments.get("expression").asText();
        try {
            double value = evaluateExpression(expression);
            return resultNode(formatNumber(value), false);
        } catch (Exception exc) {
            return resultNode("Invalid expression: " + exc.getMessage(), true);
        }
    }

    private static JsonNode resultNode(String text, boolean isError) {
        ObjectNode result = MAPPER.createObjectNode();
        ArrayNode content = MAPPER.createArrayNode();
        ObjectNode item = content.addObject();
        item.put("type", "text");
        item.put("text", text);
        result.set("content", content);
        result.put("isError", isError);
        return result;
    }

    private static String formatNumber(double value) {
        if (Math.floor(value) == value && !Double.isInfinite(value)) {
            return Long.toString((long) value);
        }
        return Double.toString(value);
    }

    private static void smokeTest() throws Exception {
        List<JsonNode> requests = List.of(
                makeRequest(1, "initialize", MAPPER.createObjectNode().put("protocolVersion", "2024-11-05").set("capabilities", MAPPER.createObjectNode())),
                makeRequest(null, "notifications/initialized", MAPPER.createObjectNode()),
                makeRequest(2, "tools/list", MAPPER.createObjectNode()),
                makeRequest(3, "tools/call", MAPPER.createObjectNode()
                        .put("name", "get_current_time")
                        .set("arguments", MAPPER.createObjectNode().put("timezone", "UTC"))),
                makeRequest(4, "tools/call", MAPPER.createObjectNode()
                        .put("name", "calculate")
                        .set("arguments", MAPPER.createObjectNode().put("expression", "(2 + 3) * 4")))
        );

        ByteArrayOutputStream stdin = new ByteArrayOutputStream();
        for (JsonNode req : requests) {
            byte[] body = MAPPER.writeValueAsBytes(req);
            stdin.write(("Content-Length: " + body.length + "\r\n\r\n").getBytes(java.nio.charset.StandardCharsets.UTF_8));
            stdin.write(body);
        }
        ByteArrayInputStream in = new ByteArrayInputStream(stdin.toByteArray());
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        serve(in, out);
        System.out.println("Smoke test output:");
        System.out.println(out.toString(java.nio.charset.StandardCharsets.UTF_8));
    }

    private static JsonNode makeRequest(Integer id, String method, JsonNode params) {
        ObjectNode req = MAPPER.createObjectNode();
        req.put("jsonrpc", "2.0");
        if (id != null) {
            req.put("id", id);
        } else {
            req.set("id", null);
        }
        req.put("method", method);
        req.set("params", params);
        return req;
    }

    private static double evaluateExpression(String expression) {
        return new Parser(expression).parseExpression();
    }

    private static class Parser {
        private final String s;
        private int pos;

        Parser(String s) {
            this.s = s;
        }

        double parseExpression() {
            double value = parseTerm();
            while (true) {
                skipWhitespace();
                if (match('+')) {
                    value += parseTerm();
                } else if (match('-')) {
                    value -= parseTerm();
                } else {
                    break;
                }
            }
            return value;
        }

        double parseTerm() {
            double value = parseFactor();
            while (true) {
                skipWhitespace();
                if (match('*')) {
                    value *= parseFactor();
                } else if (match('/')) {
                    value /= parseFactor();
                } else {
                    break;
                }
            }
            return value;
        }

        double parseFactor() {
            skipWhitespace();
            if (match('(')) {
                double value = parseExpression();
                skipWhitespace();
                if (!match(')')) {
                    throw new IllegalArgumentException("missing ')'");
                }
                return value;
            }
            return parseNumber();
        }

        double parseNumber() {
            skipWhitespace();
            int start = pos;
            while (pos < s.length() && (Character.isDigit(s.charAt(pos)) || s.charAt(pos) == '.')) {
                pos++;
            }
            if (start == pos) {
                throw new IllegalArgumentException("expected number");
            }
            return Double.parseDouble(s.substring(start, pos));
        }

        void skipWhitespace() {
            while (pos < s.length() && Character.isWhitespace(s.charAt(pos))) {
                pos++;
            }
        }

        boolean match(char expected) {
            skipWhitespace();
            if (pos < s.length() && s.charAt(pos) == expected) {
                pos++;
                return true;
            }
            return false;
        }
    }
}
