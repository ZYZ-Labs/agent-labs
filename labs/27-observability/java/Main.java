import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.time.Instant;
import java.util.Map;
import java.util.UUID;

public class Main {
    private static final ObjectMapper mapper = new ObjectMapper();

    public static void main(String[] args) throws Exception {
        OpenAiClient client = new OpenAiClient();
        String question = "Explain observability in one sentence.";
        System.out.println("User: " + question);
        String answer = runObservableAgent(client, question);
        System.out.println("Assistant: " + answer);
    }

    private static String runObservableAgent(OpenAiClient client, String userMessage) throws Exception {
        String traceId = UUID.randomUUID().toString();
        return span("agent.run", traceId, Map.of("input_length", userMessage.length()), () -> {
            try {
                ObjectNode response = span("llm.call", traceId, Map.of(), () -> {
                    ObjectNode payload = client.buildMessages(userMessage);
                    payload.put("max_tokens", 200);
                    payload.put("temperature", 0.0);
                    return client.chatCompletion(payload);
                });
                logUsage(traceId, response);

                JsonNode message = client.extractMessage(response);
                String content = message.has("content") ? message.get("content").asText() : "";

                if (message.has("tool_calls")) {
                    for (JsonNode tc : message.get("tool_calls")) {
                        String name = tc.get("function").get("name").asText();
                        span("tool.execute", traceId, Map.of("tool_name", name), () -> null);
                    }
                }

                logStructured("INFO", traceId, "agent.run", Map.of("event", "agent.response", "response", content));
                return content;
            } catch (Exception e) {
                throw new RuntimeException(e);
            }
        });
    }

    private static interface ThrowingSupplier<T> {
        T get() throws Exception;
    }

    private static <T> T span(String name, String traceId, Map<String, Object> attrs, ThrowingSupplier<T> fn) throws Exception {
        logStructured("INFO", traceId, name, merge(Map.of("event", "span.start"), attrs));
        long start = System.currentTimeMillis();
        try {
            return fn.get();
        } finally {
            long duration = System.currentTimeMillis() - start;
            logStructured("INFO", traceId, name, merge(Map.of("event", "span.end", "duration_ms", duration), attrs));
        }
    }

    private static Map<String, Object> merge(Map<String, Object> a, Map<String, Object> b) {
        Map<String, Object> result = new java.util.LinkedHashMap<>(a);
        result.putAll(b);
        return result;
    }

    private static void logUsage(String traceId, ObjectNode response) {
        JsonNode usage = response.has("usage") ? response.get("usage") : mapper.createObjectNode();
        Map<String, Object> attrs = new java.util.LinkedHashMap<>();
        attrs.put("event", "tokens.usage");
        attrs.put("prompt_tokens", usage.has("prompt_tokens") ? usage.get("prompt_tokens").asInt() : null);
        attrs.put("completion_tokens", usage.has("completion_tokens") ? usage.get("completion_tokens").asInt() : null);
        attrs.put("total_tokens", usage.has("total_tokens") ? usage.get("total_tokens").asInt() : null);
        logStructured("INFO", traceId, "usage", attrs);
    }

    private static void logStructured(String level, String traceId, String spanName, Map<String, Object> attrs) {
        Map<String, Object> payload = new java.util.LinkedHashMap<>();
        payload.put("timestamp", Instant.now().toString());
        payload.put("level", level);
        payload.put("trace_id", traceId);
        payload.put("span_name", spanName);
        payload.putAll(attrs);
        try {
            System.out.println(mapper.writeValueAsString(payload));
        } catch (Exception e) {
            e.printStackTrace();
        }
    }
}
