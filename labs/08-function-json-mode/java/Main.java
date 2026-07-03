import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.util.List;
import java.util.Map;

public class Main {

    private static final String EVENT_SCHEMA_JSON = """
            {
              "type": "object",
              "properties": {
                "name": {"type": "string"},
                "date": {"type": "string"},
                "location": {"type": "string"},
                "participants": {"type": "array", "items": {"type": "string"}}
              },
              "required": ["name", "date"]
            }
            """;

    private static final String USER_PROMPT = """
            Extract the event details from this message as JSON:

            "Join us for the AI Engineering Meetup on 2025-09-15 at the Shenzhen Hub.
            Attendees: Alice, Bob, and Carol."
            """.strip();

    public static void main(String[] args) throws Exception {
        OpenAiClient client = new OpenAiClient();
        ObjectMapper mapper = new ObjectMapper();

        Map<String, Object> jsonMode = runMode("JSON mode", () -> extractWithJsonMode(client, mapper), mapper);
        Map<String, Object> functionMode = runMode("Function calling", () -> extractWithFunctionCall(client, mapper), mapper);

        System.out.println("\n" + "=".repeat(40));
        System.out.println("Summary");
        System.out.println("=".repeat(40));
        System.out.println("JSON mode ok: " + jsonMode.get("ok"));
        System.out.println("Function call ok: " + functionMode.get("ok"));

        if (Boolean.TRUE.equals(jsonMode.get("ok")) && Boolean.TRUE.equals(functionMode.get("ok"))) {
            Map<String, Object> j = (Map<String, Object>) jsonMode.get("result");
            Map<String, Object> f = (Map<String, Object>) functionMode.get("result");
            System.out.println("\nBoth approaches returned valid events. "
                    + "Function calling gives you an explicit schema contract; "
                    + "JSON mode is simpler when you only need a shaped text response.");
            System.out.println("Names match: " + (j.get("name").equals(f.get("name")) && j.get("date").equals(f.get("date"))));
        }
    }

    private static Map<String, Object> runMode(String label, Extractor extractor, ObjectMapper mapper) throws Exception {
        System.out.println("\n" + "=".repeat(40) + "\nMode: " + label + "\n" + "=".repeat(40));
        try {
            Map<String, Object> result = extractor.extract();
            System.out.println("\n[" + label + "] parsed event:");
            System.out.println(mapper.writerWithDefaultPrettyPrinter().writeValueAsString(result));
            return Map.of("ok", true, "result", result);
        } catch (Exception exc) {
            System.out.println("\n[" + label + "] FAILED: " + exc.getMessage());
            return Map.of("ok", false, "error", exc.getMessage());
        }
    }

    private static Map<String, Object> extractWithJsonMode(OpenAiClient client, ObjectMapper mapper) throws Exception {
        ObjectNode payload = mapper.createObjectNode();
        ArrayNode messages = mapper.createArrayNode();

        ObjectNode system = messages.addObject();
        system.put("role", "system");
        system.put("content", "You are a helpful parser. Return ONLY a JSON object with keys: name, date, location, participants.");

        ObjectNode user = messages.addObject();
        user.put("role", "user");
        user.put("content", USER_PROMPT);

        payload.set("messages", messages);
        ObjectNode responseFormat = mapper.createObjectNode();
        responseFormat.put("type", "json_object");
        payload.set("response_format", responseFormat);
        payload.put("temperature", 0.0);
        payload.put("max_tokens", 300);

        JsonNode response = client.chatCompletion(payload);
        String raw = client.extractMessage(response).get("content").asText();
        System.out.println("\n[JSON mode] raw output:");
        System.out.println(raw);
        return validateEvent(mapper.readValue(raw, Map.class));
    }

    private static Map<String, Object> extractWithFunctionCall(OpenAiClient client, ObjectMapper mapper) throws Exception {
        ObjectNode tool = mapper.createObjectNode();
        tool.put("type", "function");
        ObjectNode function = mapper.createObjectNode();
        function.put("name", "extract_event");
        function.put("description", "Extract event details from user text.");
        function.set("parameters", mapper.readTree(EVENT_SCHEMA_JSON));
        tool.set("function", function);

        ObjectNode payload = mapper.createObjectNode();
        ArrayNode messages = mapper.createArrayNode();

        ObjectNode system = messages.addObject();
        system.put("role", "system");
        system.put("content", "Use the extract_event tool.");

        ObjectNode user = messages.addObject();
        user.put("role", "user");
        user.put("content", USER_PROMPT);

        payload.set("messages", messages);
        ArrayNode tools = mapper.createArrayNode();
        tools.add(tool);
        payload.set("tools", tools);
        ObjectNode toolChoice = mapper.createObjectNode();
        toolChoice.put("type", "function");
        ObjectNode func = toolChoice.putObject("function");
        func.put("name", "extract_event");
        payload.set("tool_choice", toolChoice);
        payload.put("temperature", 0.0);
        payload.put("max_tokens", 300);

        JsonNode response = client.chatCompletion(payload);
        JsonNode message = client.extractMessage(response);
        JsonNode toolCalls = message.get("tool_calls");
        if (toolCalls == null || toolCalls.isEmpty()) {
            throw new IllegalStateException("Model did not call the extract_event tool");
        }
        String raw = toolCalls.get(0).get("function").get("arguments").asText();
        System.out.println("\n[Function call] raw arguments:");
        System.out.println(raw);
        return validateEvent(mapper.readValue(raw, Map.class));
    }

    private static Map<String, Object> validateEvent(Map<String, Object> data) {
        if (!(data instanceof Map)) {
            throw new IllegalArgumentException("Parsed JSON is not an object");
        }
        List<String> required = List.of("name", "date");
        List<String> missing = required.stream().filter(k -> !data.containsKey(k)).toList();
        if (!missing.isEmpty()) {
            throw new IllegalArgumentException("Missing required fields: " + missing);
        }
        Object participants = data.get("participants");
        if (participants != null && !(participants instanceof List)) {
            throw new IllegalArgumentException("Field 'participants' should be an array");
        }
        return data;
    }

    @FunctionalInterface
    interface Extractor {
        Map<String, Object> extract() throws Exception;
    }
}
