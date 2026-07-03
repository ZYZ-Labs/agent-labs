import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.*;
import java.util.concurrent.TimeUnit;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class Main {

    private static final Set<String> ALLOWED_COMMANDS = Set.of(
            "systemctl restart",
            "kubectl rollout restart",
            "docker restart",
            "echo",
            "df"
    );

    public static void main(String[] args) throws Exception {
        OpenAiClient client = null;
        try {
            client = new OpenAiClient();
        } catch (IllegalStateException exc) {
            System.out.println("LLM client disabled: " + exc.getMessage());
        }

        Path logPath = args.length > 0 ? Paths.get(args[0]) : Paths.get("..", "python", "sample_app.log");
        if (!Files.exists(logPath)) {
            System.err.println("Log file not found: " + logPath);
            System.exit(1);
        }

        List<Map<String, String>> entries = parseLogs(logPath);
        System.out.println("Read " + entries.size() + " log entries from " + logPath);

        Map<String, Object> diagnosis = diagnose(entries, client);
        System.out.println("\n=== Diagnosis ===");
        System.out.println(diagnosis.get("diagnosis"));
        System.out.println("Risk: " + diagnosis.getOrDefault("risk", "unknown"));

        @SuppressWarnings("unchecked")
        List<String> commands = (List<String>) diagnosis.getOrDefault("commands", List.of());
        System.out.println("\n=== Proposed remediation commands ===");
        if (commands.isEmpty()) {
            System.out.println("No remediation commands proposed.");
            return;
        }
        for (String cmd : commands) {
            System.out.println("  - " + cmd);
        }

        Scanner scanner = new Scanner(System.in);
        System.out.print("\nExecute proposed commands? [y/n]: ");
        String answer = scanner.nextLine().strip().toLowerCase();
        if (!answer.equals("y") && !answer.equals("yes")) {
            System.out.println("Execution cancelled by operator.");
            return;
        }

        System.out.println("\nExecuting commands...");
        List<Map<String, Object>> results = executeCommands(commands);
        System.out.println("\nExecution summary:");
        System.out.println(new ObjectMapper().writerWithDefaultPrettyPrinter().writeValueAsString(results));
    }

    private static List<Map<String, String>> parseLogs(Path path) throws IOException {
        Pattern pattern = Pattern.compile("^(\\S+)\\s+(\\w+)\\s+(.+)$");
        List<Map<String, String>> entries = new ArrayList<>();
        for (String line : Files.readAllLines(path)) {
            Matcher matcher = pattern.matcher(line);
            if (matcher.matches()) {
                Map<String, String> entry = new LinkedHashMap<>();
                entry.put("ts", matcher.group(1));
                entry.put("level", matcher.group(2));
                entry.put("message", matcher.group(3));
                entries.add(entry);
            } else {
                Map<String, String> entry = new LinkedHashMap<>();
                entry.put("ts", "");
                entry.put("level", "UNKNOWN");
                entry.put("message", line);
                entries.add(entry);
            }
        }
        return entries;
    }

    private static Map<String, Object> summarizeErrors(List<Map<String, String>> entries) {
        List<String> errorMessages = new ArrayList<>();
        Map<String, Integer> counts = new HashMap<>();
        for (Map<String, String> e : entries) {
            String level = e.get("level");
            counts.put(level, counts.getOrDefault(level, 0) + 1);
            if ("ERROR".equals(level) || "FATAL".equals(level)) {
                errorMessages.add(e.get("message"));
            }
        }
        return Map.of("error_messages", errorMessages, "counts", counts);
    }

    private static Map<String, Object> diagnose(List<Map<String, String>> entries, OpenAiClient client) throws Exception {
        if (client == null) {
            return Map.of(
                    "diagnosis", "Database appears unreachable (multiple connection timeouts and 503 errors). Disk usage is also elevated.",
                    "commands", List.of(
                            "echo 'Checking database service status...'",
                            "systemctl restart postgresql",
                            "echo 'Monitoring disk usage...'",
                            "df -h"
                    ),
                    "risk", "medium"
            );
        }
        StringBuilder logText = new StringBuilder();
        for (Map<String, String> e : entries) {
            logText.append(e.get("level")).append(": ").append(e.get("message")).append("\n");
        }
        String prompt = "You are an SRE agent. Diagnose the following logs and respond with JSON in this exact shape:\n{\n  \"diagnosis\": \"short diagnosis\",\n  \"commands\": [\"command1\", \"command2\"],\n  \"risk\": \"low|medium|high\"\n}\n\nLogs:\n" + logText;

        ObjectMapper mapper = new ObjectMapper();
        ObjectNode payload = mapper.createObjectNode();
        ArrayNode messages = mapper.createArrayNode();
        ObjectNode user = messages.addObject();
        user.put("role", "user");
        user.put("content", prompt);
        payload.set("messages", messages);
        payload.put("temperature", 0.2);
        payload.put("max_tokens", 200);
        ObjectNode responseFormat = mapper.createObjectNode();
        responseFormat.put("type", "json_object");
        payload.set("response_format", responseFormat);

        JsonNode response = client.chatCompletion(payload);
        String content = client.extractMessage(response).get("content").asText();
        try {
            return mapper.readValue(content, Map.class);
        } catch (Exception exc) {
            return Map.of("diagnosis", content, "commands", List.of(), "risk", "unknown");
        }
    }

    private static boolean isCommandAllowed(String command) {
        for (String prefix : ALLOWED_COMMANDS) {
            if (command.strip().startsWith(prefix)) {
                return true;
            }
        }
        return false;
    }

    private static List<Map<String, Object>> executeCommands(List<String> commands) {
        List<Map<String, Object>> results = new ArrayList<>();
        for (String cmd : commands) {
            System.out.println("  $ " + cmd);
            if (!isCommandAllowed(cmd)) {
                System.out.println("    -> SKIPPED (not in allowlist)");
                results.add(Map.of("command", cmd, "status", "skipped", "output", ""));
                continue;
            }
            try {
                ProcessBuilder pb = new ProcessBuilder("sh", "-c", cmd);
                pb.redirectErrorStream(true);
                Process process = pb.start();
                if (!process.waitFor(10, TimeUnit.SECONDS)) {
                    process.destroyForcibly();
                    results.add(Map.of("command", cmd, "status", "error", "output", "timed out"));
                    System.out.println("    -> ERROR: timed out");
                    continue;
                }
                String output = new String(process.getInputStream().readAllBytes()).strip();
                if (output.isEmpty()) {
                    output = "<no output>";
                }
                System.out.println("    -> exit " + process.exitValue());
                results.add(Map.of("command", cmd, "status", "executed", "output", output));
            } catch (Exception exc) {
                System.out.println("    -> ERROR: " + exc.getMessage());
                results.add(Map.of("command", cmd, "status", "error", "output", exc.getMessage()));
            }
        }
        return results;
    }
}
