import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.util.*;

public class Main {

    private static final Map<String, AgentConfig> AGENTS = Map.of(
            "coding", new AgentConfig(
                    "You are a coding assistant. Answer the user's programming question with concise code and explanation.",
                    "Use Python functions and add type hints for clarity."
            ),
            "writing", new AgentConfig(
                    "You are a writing assistant. Improve clarity, grammar, and tone.",
                    "Use short sentences and active voice."
            ),
            "math", new AgentConfig(
                    "You are a math assistant. Solve the problem step by step.",
                    "Break the problem into smaller equations."
            )
    );

    public static void main(String[] args) throws Exception {
        OpenAiClient client = null;
        try {
            client = new OpenAiClient();
        } catch (IllegalStateException exc) {
            System.out.println("LLM client disabled: " + exc.getMessage());
        }

        MultiAgentSystem system = new MultiAgentSystem(client);
        String request = "How do I write a Python function that retries a failing operation?";
        System.out.println("User request: " + request);
        Map<String, Object> result = system.run(request);
        System.out.println("\nRouted to: " + result.get("topics"));
        System.out.println("\nSpecialist answers:");
        @SuppressWarnings("unchecked")
        List<Map<String, String>> responses = (List<Map<String, String>>) result.get("responses");
        for (Map<String, String> r : responses) {
            String answer = r.get("answer");
            System.out.println("  [" + r.get("topic") + "] " + answer.substring(0, Math.min(answer.length(), 200)) + "...");
        }
        System.out.println("\nFinal aggregated answer:");
        System.out.println(result.get("final_answer"));
    }

    static class AgentConfig {
        final String system;
        final String fallback;

        AgentConfig(String system, String fallback) {
            this.system = system;
            this.fallback = fallback;
        }
    }

    static class MultiAgentSystem {
        private final OpenAiClient client;

        MultiAgentSystem(OpenAiClient client) {
            this.client = client;
        }

        List<String> route(String request) throws Exception {
            if (client == null) {
                System.out.println("No LLM; routing by keyword fallback");
                String lowered = request.toLowerCase();
                List<String> topics = new ArrayList<>();
                if (containsAny(lowered, "code", "python", "function", "error")) {
                    topics.add("coding");
                }
                if (containsAny(lowered, "write", "essay", "grammar", "draft")) {
                    topics.add("writing");
                }
                if (containsAny(lowered, "math", "calculate", "equation", "sum")) {
                    topics.add("math");
                }
                return topics.isEmpty() ? List.of("writing") : topics;
            }

            ObjectMapper mapper = new ObjectMapper();
            ObjectNode payload = mapper.createObjectNode();
            ArrayNode messages = mapper.createArrayNode();
            ObjectNode system = messages.addObject();
            system.put("role", "system");
            system.put("content", "You are a router. Given a user request, choose one or more specialist topics from: coding, writing, math. Reply with a JSON array of strings only, e.g. [\"coding\"].");
            ObjectNode user = messages.addObject();
            user.put("role", "user");
            user.put("content", request);
            payload.set("messages", messages);
            payload.put("temperature", 0.0);
            payload.put("max_tokens", 50);
            ObjectNode responseFormat = mapper.createObjectNode();
            responseFormat.put("type", "json_object");
            payload.set("response_format", responseFormat);

            JsonNode response = client.chatCompletion(payload);
            String content = client.extractMessage(response).get("content").asText();
            try {
                JsonNode parsed = mapper.readTree(content);
                if (parsed.isArray()) {
                    List<String> topics = new ArrayList<>();
                    for (JsonNode node : parsed) {
                        String t = node.asText();
                        if (AGENTS.containsKey(t)) {
                            topics.add(t);
                        }
                    }
                    return topics.isEmpty() ? List.of("writing") : topics;
                }
                if (parsed.isObject()) {
                    JsonNode arr = parsed.get("topics");
                    if (arr != null && arr.isArray()) {
                        List<String> topics = new ArrayList<>();
                        for (JsonNode node : arr) {
                            String t = node.asText();
                            if (AGENTS.containsKey(t)) {
                                topics.add(t);
                            }
                        }
                        return topics.isEmpty() ? List.of("writing") : topics;
                    }
                }
            } catch (Exception ignored) {
            }
            return List.of("writing");
        }

        Map<String, String> worker(String topic, String request) throws Exception {
            AgentConfig cfg = AGENTS.get(topic);
            String answer;
            if (client == null) {
                answer = cfg.fallback;
            } else {
                ObjectMapper mapper = new ObjectMapper();
                ObjectNode payload = mapper.createObjectNode();
                ArrayNode messages = mapper.createArrayNode();
                ObjectNode system = messages.addObject();
                system.put("role", "system");
                system.put("content", cfg.system);
                ObjectNode user = messages.addObject();
                user.put("role", "user");
                user.put("content", request);
                payload.set("messages", messages);
                payload.put("temperature", 0.3);
                payload.put("max_tokens", 200);

                JsonNode response = client.chatCompletion(payload);
                answer = client.extractMessage(response).get("content").asText().strip();
            }
            return Map.of("topic", topic, "answer", answer);
        }

        String aggregate(String request, List<Map<String, String>> responses) throws Exception {
            if (client == null) {
                List<String> parts = new ArrayList<>();
                for (Map<String, String> r : responses) {
                    parts.add("### " + r.get("topic") + "\n" + r.get("answer"));
                }
                return String.join("\n\n", parts);
            }

            List<String> parts = new ArrayList<>();
            for (Map<String, String> r : responses) {
                parts.add("### " + r.get("topic") + "\n" + r.get("answer"));
            }
            String combined = String.join("\n\n", parts);
            ObjectMapper mapper = new ObjectMapper();
            ObjectNode payload = mapper.createObjectNode();
            ArrayNode messages = mapper.createArrayNode();
            ObjectNode system = messages.addObject();
            system.put("role", "system");
            system.put("content", "You are an aggregator. Combine the specialist answers into a single coherent response.");
            ObjectNode user = messages.addObject();
            user.put("role", "user");
            user.put("content", "User request: " + request + "\n\nSpecialist answers:\n" + combined + "\n\nProvide a final answer.");
            payload.set("messages", messages);
            payload.put("temperature", 0.3);
            payload.put("max_tokens", 300);

            JsonNode response = client.chatCompletion(payload);
            return client.extractMessage(response).get("content").asText().strip();
        }

        Map<String, Object> run(String request) throws Exception {
            List<String> topics = route(request);
            System.out.println("Routed to: " + topics);
            List<Map<String, String>> responses = new ArrayList<>();
            for (String topic : topics) {
                responses.add(worker(topic, request));
            }
            String finalAnswer = aggregate(request, responses);
            return Map.of("topics", topics, "responses", responses, "final_answer", finalAnswer);
        }

        private boolean containsAny(String text, String... keywords) {
            for (String kw : keywords) {
                if (text.contains(kw)) {
                    return true;
                }
            }
            return false;
        }
    }
}
