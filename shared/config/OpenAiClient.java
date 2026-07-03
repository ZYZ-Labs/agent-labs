package com.agentlabs;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.Map;

/**
 * Shared OpenAI-compatible client wrapper for Java labs.
 * Reads configuration from environment variables.
 */
public class OpenAiClient {

    private final String apiKey;
    private final String baseUrl;
    private final String model;
    private final HttpClient httpClient;
    private final ObjectMapper objectMapper = new ObjectMapper();
    private final int maxRetries;

    public OpenAiClient() {
        this.apiKey = System.getenv().getOrDefault("OPENAI_API_KEY", "");
        this.baseUrl = System.getenv().getOrDefault("OPENAI_BASE_URL", "https://api.openai.com/v1").replaceAll("/$", "");
        this.model = System.getenv().getOrDefault("OPENAI_MODEL", "gpt-4o-mini");
        this.httpClient = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(60))
                .build();
        this.maxRetries = 3;
        if (apiKey.isEmpty() && !baseUrl.startsWith("http://localhost")) {
            throw new IllegalStateException("OPENAI_API_KEY is required for non-local endpoints");
        }
    }

    public JsonNode chatCompletion(ObjectNode payload) throws IOException, InterruptedException {
        String url = baseUrl + "/chat/completions";
        if (!payload.has("model")) {
            payload.put("model", model);
        }
        if (!payload.has("stream")) {
            payload.put("stream", false);
        }

        String body = objectMapper.writeValueAsString(payload);
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .header("Content-Type", "application/json")
                .header("Authorization", "Bearer " + apiKey)
                .POST(HttpRequest.BodyPublishers.ofString(body))
                .build();

        IOException lastIoException = null;
        InterruptedException lastInterruptedException = null;
        for (int attempt = 1; attempt <= maxRetries; attempt++) {
            try {
                HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
                if (response.statusCode() >= 200 && response.statusCode() < 300) {
                    return objectMapper.readTree(response.body());
                }
                if (attempt == maxRetries) {
                    throw new IOException("HTTP " + response.statusCode() + ": " + response.body());
                }
            } catch (IOException e) {
                lastIoException = e;
                if (attempt == maxRetries) break;
                Thread.sleep(attempt * 1000L);
            }
        }
        if (lastInterruptedException != null) {
            throw lastInterruptedException;
        }
        throw lastIoException != null ? lastIoException : new IOException("Max retries exceeded");
    }

    public JsonNode buildMessages(String userContent) {
        ObjectNode root = objectMapper.createObjectNode();
        ArrayNode messages = root.putArray("messages");
        ObjectNode userMessage = messages.addObject();
        userMessage.put("role", "user");
        userMessage.put("content", userContent);
        return root;
    }

    public JsonNode extractMessage(JsonNode response) {
        return response.get("choices").get(0).get("message");
    }
}
