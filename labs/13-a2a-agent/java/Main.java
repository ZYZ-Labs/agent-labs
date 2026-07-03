import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpHandler;
import com.sun.net.httpserver.HttpServer;

import java.io.*;
import java.net.InetSocketAddress;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.TimeUnit;

public class Main {

    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final String DEFAULT_URL = "http://localhost:8123";

    private static final ObjectNode AGENT_CARD = MAPPER.createObjectNode()
            .put("name", "agent-labs-echo-agent")
            .put("description", "A minimal A2A agent that echoes input after a short delay.")
            .put("url", DEFAULT_URL)
            .put("version", "0.1.0")
            .set("capabilities", MAPPER.createObjectNode()
                    .put("streaming", false)
                    .put("pushNotifications", false))
            .set("skills", MAPPER.createArrayNode()
                    .add(MAPPER.createObjectNode()
                            .put("id", "echo")
                            .put("name", "Echo")
                            .put("description", "Returns the input text as the task result.")));

    private static final Map<String, ObjectNode> TASKS = new ConcurrentHashMap<>();

    public static void main(String[] args) throws Exception {
        String baseUrl = System.getenv().getOrDefault("A2A_AGENT_URL", DEFAULT_URL).replaceAll("/+$", "");
        HttpServer ownServer = null;

        try {
            try {
                fetchAgentCard(baseUrl);
                System.out.println("Using existing agent at " + baseUrl);
            } catch (Exception exc) {
                System.out.println("No agent found at " + baseUrl + "; starting local agent");
                ownServer = startServer("localhost", 8123);
                for (int i = 0; i < 20; i++) {
                    try {
                        fetchAgentCard(baseUrl);
                        break;
                    } catch (Exception ignored) {
                        TimeUnit.MILLISECONDS.sleep(100);
                    }
                }
            }

            ObjectNode card = fetchAgentCard(baseUrl);
            System.out.println("\n[Agent Card]");
            System.out.println(MAPPER.writerWithDefaultPrettyPrinter().writeValueAsString(card));

            ObjectNode task = submitTask(baseUrl, "Hello from the A2A client!");
            System.out.println("\n[Submitted Task]");
            System.out.println(MAPPER.writerWithDefaultPrettyPrinter().writeValueAsString(task));

            ObjectNode finalTask = pollTask(baseUrl, task.get("id").asText());
            System.out.println("\n[Final Task]");
            System.out.println(MAPPER.writerWithDefaultPrettyPrinter().writeValueAsString(finalTask));
        } finally {
            if (ownServer != null) {
                ownServer.stop(0);
            }
        }
    }

    private static HttpServer startServer(String host, int port) throws IOException {
        HttpServer server = HttpServer.create(new InetSocketAddress(host, port), 0);
        server.createContext("/.well-known/agent.json", new AgentHandler());
        server.createContext("/tasks", new AgentHandler());
        server.createContext("/tasks/", new AgentHandler());
        server.setExecutor(java.util.concurrent.Executors.newFixedThreadPool(4));
        server.start();
        return server;
    }

    static class AgentHandler implements HttpHandler {
        @Override
        public void handle(HttpExchange exchange) throws IOException {
            String method = exchange.getRequestMethod();
            URI uri = exchange.getRequestURI();
            String path = uri.getPath();
            try {
                if ("GET".equals(method) && "/.well-known/agent.json".equals(path)) {
                    sendJson(exchange, 200, AGENT_CARD);
                } else if ("GET".equals(method) && path.startsWith("/tasks/")) {
                    String taskId = path.substring("/tasks/".length());
                    ObjectNode task = TASKS.get(taskId);
                    if (task != null) {
                        sendJson(exchange, 200, task);
                    } else {
                        sendJson(exchange, 404, error("Task not found"));
                    }
                } else if ("POST".equals(method) && "/tasks/send".equals(path)) {
                    String body = new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8);
                    ObjectNode payload = body.isEmpty() ? MAPPER.createObjectNode() : (ObjectNode) MAPPER.readTree(body);
                    ObjectNode message = (ObjectNode) payload.get("message");
                    ObjectNode task = createTask(message);
                    sendJson(exchange, 200, task);
                } else {
                    sendJson(exchange, 404, error("Not found"));
                }
            } catch (Exception exc) {
                sendJson(exchange, 500, error(exc.getMessage()));
            }
        }
    }

    private static ObjectNode createTask(ObjectNode message) {
        String taskId = UUID.randomUUID().toString();
        long now = System.currentTimeMillis() / 1000.0;
        ObjectNode task = MAPPER.createObjectNode();
        task.put("id", taskId);
        task.set("status", MAPPER.createObjectNode().put("state", "submitted").put("timestamp", now));
        ArrayNode messages = MAPPER.createArrayNode();
        messages.add(message);
        task.set("messages", messages);
        task.set("artifacts", MAPPER.createArrayNode());
        TASKS.put(taskId, task);

        new Thread(() -> {
            try {
                Thread.sleep(2000);
                synchronized (task) {
                    task.set("status", MAPPER.createObjectNode().put("state", "working").put("timestamp", System.currentTimeMillis() / 1000.0));
                }
                Thread.sleep(2000);
                synchronized (task) {
                    ObjectNode status = MAPPER.createObjectNode();
                    status.put("state", "completed");
                    status.put("timestamp", System.currentTimeMillis() / 1000.0);
                    ObjectNode msg = status.putObject("message");
                    msg.put("role", "agent");
                    ArrayNode parts = MAPPER.createArrayNode();
                    ObjectNode part = parts.addObject();
                    part.put("type", "text");
                    JsonNode firstPart = message.get("parts").get(0);
                    part.put("text", "Echo: " + firstPart.get("text").asText());
                    msg.set("parts", parts);
                    task.set("status", status);
                }
            } catch (InterruptedException ignored) {
            }
        }).start();
        return task;
    }

    private static void sendJson(HttpExchange exchange, int status, JsonNode payload) throws IOException {
        byte[] body = MAPPER.writeValueAsBytes(payload);
        exchange.getResponseHeaders().set("Content-Type", "application/json");
        exchange.sendResponseHeaders(status, body.length);
        try (OutputStream out = exchange.getResponseBody()) {
            out.write(body);
        }
    }

    private static ObjectNode error(String message) {
        return MAPPER.createObjectNode().put("error", message);
    }

    private static ObjectNode fetchAgentCard(String baseUrl) throws Exception {
        HttpClient client = HttpClient.newHttpClient();
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(baseUrl + "/.well-known/agent.json"))
                .GET()
                .build();
        HttpResponse<String> response = client.send(request, HttpResponse.BodyHandlers.ofString());
        if (response.statusCode() >= 300) {
            throw new IOException("HTTP " + response.statusCode());
        }
        return (ObjectNode) MAPPER.readTree(response.body());
    }

    private static ObjectNode submitTask(String baseUrl, String text) throws Exception {
        HttpClient client = HttpClient.newHttpClient();
        ObjectNode payload = MAPPER.createObjectNode();
        ObjectNode message = payload.putObject("message");
        message.put("role", "user");
        ArrayNode parts = MAPPER.createArrayNode();
        ObjectNode part = parts.addObject();
        part.put("type", "text");
        part.put("text", text);
        message.set("parts", parts);

        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(baseUrl + "/tasks/send"))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(MAPPER.writeValueAsString(payload)))
                .build();
        HttpResponse<String> response = client.send(request, HttpResponse.BodyHandlers.ofString());
        return (ObjectNode) MAPPER.readTree(response.body());
    }

    private static ObjectNode getTask(String baseUrl, String taskId) throws Exception {
        HttpClient client = HttpClient.newHttpClient();
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(baseUrl + "/tasks/" + taskId))
                .GET()
                .build();
        HttpResponse<String> response = client.send(request, HttpResponse.BodyHandlers.ofString());
        return (ObjectNode) MAPPER.readTree(response.body());
    }

    private static ObjectNode pollTask(String baseUrl, String taskId) throws Exception {
        long deadline = System.currentTimeMillis() + 30_000;
        while (System.currentTimeMillis() < deadline) {
            ObjectNode task = getTask(baseUrl, taskId);
            String state = task.get("status").get("state").asText();
            System.out.println("Task " + taskId.substring(0, 8) + " state: " + state);
            if ("completed".equals(state) || "failed".equals(state)) {
                return task;
            }
            Thread.sleep(500);
        }
        throw new RuntimeException("Task " + taskId + " did not complete within 30s");
    }
}
