import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.io.*;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class Main {

    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final Map<String, String> DOCUMENTS = new ConcurrentHashMap<>();

    public static void main(String[] args) throws Exception {
        if (args.length > 0 && "--smoke".equals(args[0])) {
            smokeTest();
        } else {
            serve(System.in, System.out);
        }
    }

    private static void serve(InputStream in, OutputStream out) throws Exception {
        System.err.println("LSP server ready on stdio");
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
            String method = request.get("method").asText();
            JsonNode response = handleRequest(request);
            if (response != null) {
                writeMessage(out, response);
            }
            if ("exit".equals(method)) {
                break;
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
        return MAPPER.readTree(new String(body, StandardCharsets.UTF_8));
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
        return buf.toString(StandardCharsets.UTF_8);
    }

    private static void writeMessage(OutputStream out, JsonNode message) throws Exception {
        byte[] body = MAPPER.writeValueAsBytes(message);
        out.write(("Content-Length: " + body.length + "\r\n\r\n").getBytes(StandardCharsets.UTF_8));
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
                    .set("capabilities", MAPPER.createObjectNode()
                            .set("textDocumentSync", MAPPER.createObjectNode().put("openClose", true).put("change", 0))
                            .put("definitionProvider", true))
                    .set("serverInfo", MAPPER.createObjectNode().put("name", "agent-labs-lsp").put("version", "0.1.0")));
            case "initialized" -> null;
            case "textDocument/didOpen" -> {
                JsonNode textDocument = params.get("textDocument");
                updateDocument(textDocument.get("uri").asText(), textDocument.get("text").asText());
                yield null;
            }
            case "textDocument/definition" -> {
                JsonNode textDocument = params.get("textDocument");
                JsonNode position = params.get("position");
                JsonNode location = findDefinition(textDocument.get("uri").asText(), position);
                yield makeResponse(reqId, location);
            }
            case "shutdown" -> makeResponse(reqId, null);
            case "exit" -> null;
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

    private static void updateDocument(String uri, String text) {
        DOCUMENTS.put(uri, text);
    }

    private static JsonNode findDefinition(String uri, JsonNode position) {
        String text = DOCUMENTS.getOrDefault(uri, "");
        String[] lines = text.split("\n", -1);
        int lineIdx = position.has("line") ? position.get("line").asInt() : 0;
        int charIdx = position.has("character") ? position.get("character").asInt() : 0;
        if (!(0 <= lineIdx && lineIdx < lines.length)) {
            return null;
        }
        String line = lines[lineIdx];
        String after = charIdx < line.length() ? line.substring(charIdx) : "";
        Matcher matcher = Pattern.compile("[A-Za-z0-9_]+").matcher(after);
        if (!matcher.find()) {
            return null;
        }
        String word = matcher.group(0);
        Pattern defPattern = Pattern.compile("^def\\s+" + Pattern.quote(word) + "\\s*\\(", Pattern.MULTILINE);
        for (Map.Entry<String, String> entry : DOCUMENTS.entrySet()) {
            Matcher defMatcher = defPattern.matcher(entry.getValue());
            if (defMatcher.find()) {
                int startLine = entry.getValue().substring(0, defMatcher.start()).split("\n", -1).length - 1;
                ObjectNode location = MAPPER.createObjectNode();
                location.put("uri", entry.getKey());
                ObjectNode range = location.putObject("range");
                ObjectNode start = range.putObject("start");
                start.put("line", startLine);
                start.put("character", 0);
                ObjectNode end = range.putObject("end");
                end.put("line", startLine);
                end.put("character", ("def " + word + "(").length());
                return location;
            }
        }
        return null;
    }

    private static void smokeTest() throws Exception {
        String sampleCode = String.join("\n", List.of(
                "def greet(name):",
                "    return f\"Hello, {name}!\"",
                "",
                "print(greet(\"world\"))"
        ));
        String uri = "file:///tmp/sample.py";

        List<JsonNode> requests = List.of(
                makeRequest(1, "initialize", MAPPER.createObjectNode()
                        .put("processId", (String) null).set("rootUri", null).set("capabilities", MAPPER.createObjectNode())),
                makeRequest(null, "initialized", MAPPER.createObjectNode()),
                makeRequest(null, "textDocument/didOpen", MAPPER.createObjectNode()
                        .set("textDocument", MAPPER.createObjectNode()
                                .put("uri", uri)
                                .put("languageId", "python")
                                .put("text", sampleCode))),
                makeRequest(2, "textDocument/definition", MAPPER.createObjectNode()
                        .set("textDocument", MAPPER.createObjectNode().put("uri", uri))
                        .set("position", MAPPER.createObjectNode().put("line", 3).put("character", 6))),
                makeRequest(3, "shutdown", MAPPER.createObjectNode()),
                makeRequest(null, "exit", MAPPER.createObjectNode())
        );

        ByteArrayOutputStream stdin = new ByteArrayOutputStream();
        for (JsonNode req : requests) {
            byte[] body = MAPPER.writeValueAsBytes(req);
            stdin.write(("Content-Length: " + body.length + "\r\n\r\n").getBytes(StandardCharsets.UTF_8));
            stdin.write(body);
        }
        ByteArrayInputStream in = new ByteArrayInputStream(stdin.toByteArray());
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        serve(in, out);
        System.out.println("Smoke test output:");
        System.out.println(out.toString(StandardCharsets.UTF_8));
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
}
