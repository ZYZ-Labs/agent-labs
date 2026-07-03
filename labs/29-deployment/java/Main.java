import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;

import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.util.Map;

public class Main {
    private static final ObjectMapper mapper = new ObjectMapper();
    private static String apiKey;

    public static void main(String[] args) throws Exception {
        apiKey = System.getenv().getOrDefault("OPENAI_API_KEY", "");
        if (apiKey.isEmpty()) {
            System.err.println("Warning: OPENAI_API_KEY is not set. /chat will return 503 until configured.");
        }

        OpenAiClient client = new OpenAiClient();
        int port = Integer.parseInt(System.getenv().getOrDefault("PORT", "8000"));
        HttpServer server = HttpServer.create(new InetSocketAddress(port), 0);

        server.createContext("/health", exchange -> {
            sendJson(exchange, 200, mapper.writeValueAsString(Map.of("status", "ok", "configured", !apiKey.isEmpty())));
        });

        server.createContext("/chat", exchange -> {
            if (!"POST".equalsIgnoreCase(exchange.getRequestMethod())) {
                sendJson(exchange, 405, mapper.writeValueAsString(Map.of("error", "Method not allowed")));
                return;
            }
            if (apiKey.isEmpty()) {
                sendJson(exchange, 503, mapper.writeValueAsString(Map.of("error", "OPENAI_API_KEY is not configured")));
                return;
            }
            JsonNode req = mapper.readTree(exchange.getRequestBody());
            String message = req.has("message") ? req.get("message").asText() : "";
            String model = req.has("model") ? req.get("model").asText() : "";
            if (message.isBlank()) {
                sendJson(exchange, 400, mapper.writeValueAsString(Map.of("error", "message is required")));
                return;
            }
            try {
                ObjectNode payload = client.buildMessages(message);
                if (!model.isEmpty()) payload.put("model", model);
                payload.put("max_tokens", 300);
                payload.put("temperature", 0.0);
                ObjectNode response = client.chatCompletion(payload);
                String reply = client.extractMessage(response).get("content").asText();
                sendJson(exchange, 200, mapper.writeValueAsString(Map.of("reply", reply, "model", model.isEmpty() ? "gpt-4o-mini" : model)));
            } catch (Exception e) {
                sendJson(exchange, 502, mapper.writeValueAsString(Map.of("error", "Upstream error: " + e.getMessage())));
            }
        });

        server.start();
        System.out.println("Server listening on :" + port);
    }

    private static void sendJson(HttpExchange exchange, int status, String body) throws IOException {
        exchange.getResponseHeaders().set("Content-Type", "application/json");
        byte[] bytes = body.getBytes();
        exchange.sendResponseHeaders(status, bytes.length);
        try (OutputStream os = exchange.getResponseBody()) {
            os.write(bytes);
        }
    }
}
