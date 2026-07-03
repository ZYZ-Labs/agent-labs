import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.io.*;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.Iterator;
import java.util.List;
import java.util.Map;

public class Main {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    public static void main(String[] args) throws Exception {
        String serverScriptEnv = System.getenv("MCP_SERVER_SCRIPT");
        Path script;
        if (serverScriptEnv != null) {
            script = Paths.get(serverScriptEnv);
        } else {
            script = Paths.get("..", "09-mcp-server-stdio", "python", "main.py").toAbsolutePath().normalize();
        }
        if (!Files.exists(script)) {
            throw new FileNotFoundException("MCP server script not found: " + script);
        }

        MCPStdioClient client = new MCPStdioClient(script);
        try {
            client.connect();
            JsonNode initResponse = client.initialize();
            System.out.println("[initialize] " + MAPPER.writerWithDefaultPrettyPrinter().writeValueAsString(initResponse.get("result")));

            JsonNode tools = client.listTools();
            System.out.println("\n[tools]");
            for (JsonNode tool : tools) {
                System.out.println("  - " + tool.get("name").asText() + ": " + tool.get("description").asText(""));
            }

            JsonNode result = client.callTool("calculate", Map.of("expression", "(10 + 5) / 3"));
            System.out.println("\n[tools/call calculate]");
            for (JsonNode item : result.get("content")) {
                System.out.println("  " + item.get("text").asText());
            }

            demoSse();
        } finally {
            client.disconnect();
        }
    }

    private static void demoSse() {
        byte[] rawStream = (
                ":heartbeat\n\n"
                        + "event: message\n"
                        + "data: {\"tool\": \"calculate\", \"args\": {\"expression\": \"1+1\"}}\n\n"
                        + "event: status\n"
                        + "data: processing\n\n"
                        + "event: done\n"
                        + "data: finished\n\n"
        ).getBytes(StandardCharsets.UTF_8);

        System.out.println("\n[SSE transport concept]");
        for (Map<String, String> event : parseSse(List.of(new String(rawStream, StandardCharsets.UTF_8).split("\n")))) {
            System.out.println("  SSE event: " + event);
        }
    }

    private static List<Map<String, String>> parseSse(List<String> lines) {
        List<Map<String, String>> events = new java.util.ArrayList<>();
        Map<String, String> current = new java.util.HashMap<>();
        for (String raw : lines) {
            String line = raw.replaceAll("\\r?\\n$", "");
            if (line.isEmpty()) {
                if (!current.isEmpty()) {
                    events.add(current);
                    current = new java.util.HashMap<>();
                }
                continue;
            }
            if (line.startsWith(":")) {
                continue;
            }
            int idx = line.indexOf(':');
            if (idx >= 0) {
                String key = line.substring(0, idx);
                String value = line.substring(idx + 1).replaceFirst("^\\s+", "");
                current.put(key, value);
            }
        }
        if (!current.isEmpty()) {
            events.add(current);
        }
        return events;
    }

    static class MCPStdioClient {
        private final Path script;
        private Process process;
        private int requestId = 0;

        MCPStdioClient(Path script) {
            this.script = script;
        }

        void connect() throws IOException {
            System.out.println("Starting MCP server: " + script);
            process = new ProcessBuilder("python", script.toString())
                    .redirectErrorStream(true)
                    .start();
        }

        void disconnect() {
            if (process != null && process.isAlive()) {
                process.getOutputStream().close();
                process.destroy();
                try {
                    if (!process.waitFor(2, java.util.concurrent.TimeUnit.SECONDS)) {
                        process.destroyForcibly();
                    }
                } catch (InterruptedException ignored) {
                    process.destroyForcibly();
                }
            }
            process = null;
        }

        private int nextId() {
            return ++requestId;
        }

        private void send(JsonNode message) throws IOException {
            byte[] body = MAPPER.writeValueAsBytes(message);
            OutputStream out = process.getOutputStream();
            out.write(("Content-Length: " + body.length + "\r\n\r\n").getBytes(StandardCharsets.UTF_8));
            out.write(body);
            out.flush();
        }

        private JsonNode recv() throws IOException {
            InputStream in = process.getInputStream();
            int contentLength = 0;
            while (true) {
                String line = readLine(in);
                if (line == null) {
                    throw new ConnectionException("Server closed stdout");
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
                throw new ConnectionException("Empty message body");
            }
            byte[] body = new byte[contentLength];
            int read = 0;
            while (read < contentLength) {
                int n = in.read(body, read, contentLength - read);
                if (n < 0) {
                    throw new ConnectionException("Server closed stdout while reading body");
                }
                read += n;
            }
            try {
                return MAPPER.readTree(new String(body, StandardCharsets.UTF_8));
            } catch (Exception exc) {
                throw new IOException("Failed to parse response: " + exc.getMessage());
            }
        }

        private String readLine(InputStream in) throws IOException {
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

        JsonNode initialize() throws IOException {
            ObjectNode init = MAPPER.createObjectNode();
            init.put("jsonrpc", "2.0");
            init.put("id", nextId());
            init.put("method", "initialize");
            ObjectNode params = init.putObject("params");
            params.put("protocolVersion", "2024-11-05");
            params.set("capabilities", MAPPER.createObjectNode());
            send(init);
            JsonNode result = recv();

            ObjectNode notification = MAPPER.createObjectNode();
            notification.put("jsonrpc", "2.0");
            notification.set("id", null);
            notification.put("method", "notifications/initialized");
            send(notification);
            return result;
        }

        JsonNode listTools() throws IOException {
            ObjectNode req = MAPPER.createObjectNode();
            req.put("jsonrpc", "2.0");
            req.put("id", nextId());
            req.put("method", "tools/list");
            send(req);
            JsonNode response = recv();
            if (response.has("error")) {
                throw new RuntimeException(response.get("error").toString());
            }
            return response.get("result").get("tools");
        }

        JsonNode callTool(String name, Map<String, Object> arguments) throws IOException {
            ObjectNode req = MAPPER.createObjectNode();
            req.put("jsonrpc", "2.0");
            req.put("id", nextId());
            req.put("method", "tools/call");
            ObjectNode params = req.putObject("params");
            params.put("name", name);
            params.set("arguments", MAPPER.valueToTree(arguments));
            send(req);
            JsonNode response = recv();
            if (response.has("error")) {
                throw new RuntimeException(response.get("error").toString());
            }
            return response.get("result");
        }
    }

    private static class ConnectionException extends IOException {
        ConnectionException(String message) {
            super(message);
        }
    }
}
