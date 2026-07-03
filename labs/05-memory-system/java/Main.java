import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

public class Main {

    public static void main(String[] args) throws Exception {
        OpenAiClient client = new OpenAiClient();
        AgentWithMemory agent = new AgentWithMemory(client);

        System.out.println("Agent: " + agent.chat("My name is Alice and I work on backend systems."));
        System.out.println("Agent: " + agent.chat("What do I work on?"));
        System.out.println("Agent: " + agent.chat("Suggest a logging strategy for my team."));
    }

    static class ShortTermMemory {
        private final List<ObjectNode> messages = new ArrayList<>();
        private final int maxMessages;

        ShortTermMemory(int maxMessages) {
            this.maxMessages = maxMessages;
        }

        void add(String role, String content) {
            ObjectMapper mapper = new ObjectMapper();
            ObjectNode msg = mapper.createObjectNode();
            msg.put("role", role);
            msg.put("content", content);
            messages.add(msg);
            if (messages.size() > maxMessages) {
                int keep = (!messages.isEmpty() && "system".equals(messages.get(0).get("role").asText())) ? 1 : 0;
                messages.subList(keep, keep + 1).clear();
            }
        }

        ArrayNode get() {
            ObjectMapper mapper = new ObjectMapper();
            ArrayNode array = mapper.createArrayNode();
            messages.forEach(array::add);
            return array;
        }
    }

    static class LongTermMemory {
        private final List<String> entries = new ArrayList<>();

        void store(String text) {
            entries.add(text);
        }

        List<String> retrieve(String query) {
            String[] queryWords = query.toLowerCase().split("\\s+");
            return entries.stream()
                    .sorted((a, b) -> Integer.compare(score(b, queryWords), score(a, queryWords)))
                    .limit(3)
                    .filter(e -> score(e, queryWords) > 0)
                    .toList();
        }

        private int score(String entry, String[] queryWords) {
            String lower = entry.toLowerCase();
            return (int) Arrays.stream(queryWords).filter(lower::contains).count();
        }
    }

    static class AgentWithMemory {
        private final OpenAiClient client;
        private final ShortTermMemory shortTerm;
        private final LongTermMemory longTerm;

        AgentWithMemory(OpenAiClient client) {
            this.client = client;
            this.shortTerm = new ShortTermMemory(12);
            this.longTerm = new LongTermMemory();
        }

        String chat(String userInput) throws Exception {
            List<String> relevant = longTerm.retrieve(userInput);
            if (!relevant.isEmpty()) {
                String context = "Relevant memory:\n" + String.join("\n", relevant.stream().map(r -> "- " + r).toList());
                shortTerm.add("system", context);
            }

            shortTerm.add("user", userInput);
            ObjectMapper mapper = new ObjectMapper();
            ObjectNode payload = mapper.createObjectNode();
            payload.set("messages", shortTerm.get());
            payload.put("max_tokens", 200);

            JsonNode response = client.chatCompletion(payload);
            String answer = client.extractMessage(response).get("content").asText();
            shortTerm.add("assistant", answer);
            longTerm.store("User: " + userInput + "\nAssistant: " + answer);
            return answer;
        }
    }
}
