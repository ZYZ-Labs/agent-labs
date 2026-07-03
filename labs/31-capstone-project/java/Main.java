import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.Map;

public class Main {
    private static final ObjectMapper mapper = new ObjectMapper();
    private static final String DEFAULT_REQUIREMENTS = """
        Build a simple REST API for a task management service.
        Users should be able to create, list, update, and delete tasks.
        Each task has a title, description, status (todo/in_progress/done), and due date.
        Store data in memory; no database is required.
        Add basic input validation.""";

    public static void main(String[] args) throws Exception {
        OpenAiClient client = new OpenAiClient();
        String requirements = System.getenv().getOrDefault("REQUIREMENTS", DEFAULT_REQUIREMENTS);
        System.out.println("Requirements:");
        System.out.println(requirements);
        System.out.println();

        Path outputDir = Paths.get("..", "output");
        Files.createDirectories(outputDir);

        System.out.println("Generating design doc...");
        String designDoc = generateDesignDoc(client, requirements);
        System.out.println("  Wrote " + writeArtifact(outputDir, "design_doc.md", designDoc));

        System.out.println("Generating API code...");
        String apiCode = generateAPICode(client, requirements, designDoc);
        System.out.println("  Wrote " + writeArtifact(outputDir, "Main.java", apiCode));

        System.out.println("Generating tests...");
        String tests = generateTests(client, apiCode);
        System.out.println("  Wrote " + writeArtifact(outputDir, "MainTest.java", tests));

        System.out.println("Generating review report...");
        String review = generateReviewReport(client, requirements, designDoc, apiCode, tests);
        System.out.println("  Wrote " + writeArtifact(outputDir, "review_report.md", review));

        System.out.println("\nCapstone artifacts generated successfully.");
    }

    private static String generateDesignDoc(OpenAiClient client, String requirements) throws Exception {
        String prompt = "You are a senior backend engineer. Write a concise design document (Markdown) for the following requirements.\n\nRequirements:\n" + requirements + "\n\nInclude:\n- Overview\n- Endpoints (method, path, description)\n- Data model\n- Assumptions and constraints\n\nRespond with Markdown only.";
        ObjectNode payload = client.buildMessages(prompt);
        payload.put("max_tokens", 1500);
        payload.put("temperature", 0.2);
        return client.extractMessage(client.chatCompletion(payload)).get("content").asText();
    }

    private static String generateAPICode(OpenAiClient client, String requirements, String designDoc) throws Exception {
        String prompt = "You are a senior backend engineer. Implement a runnable Java application using com.sun.net.httpserver for the requirements below.\n\nRequirements:\n" + requirements + "\n\nDesign document:\n" + designDoc + "\n\nGuidelines:\n- Use only the standard library.\n- Store data in memory.\n- Include input validation.\n- Do not include instructions or explanations outside the code.\n- Output a single Java file named Main.java.\n\nRespond with the full Java source code only (no Markdown fences).";
        ObjectNode payload = client.buildMessages(prompt);
        payload.put("max_tokens", 2000);
        payload.put("temperature", 0.2);
        return stripMarkdownFences(client.extractMessage(client.chatCompletion(payload)).get("content").asText());
    }

    private static String generateTests(OpenAiClient client, String apiCode) throws Exception {
        String prompt = "You are a QA engineer. Write JUnit 5 tests for the following Java application.\n\nApplication code:\n" + apiCode + "\n\nGuidelines:\n- Use JUnit 5 and java.net.http.HttpClient.\n- Cover create, list, update, and delete endpoints.\n- Include at least one validation failure test.\n- Output a single Java test file named MainTest.java.\n\nRespond with the full Java test source code only (no Markdown fences).";
        ObjectNode payload = client.buildMessages(prompt);
        payload.put("max_tokens", 2000);
        payload.put("temperature", 0.2);
        return stripMarkdownFences(client.extractMessage(client.chatCompletion(payload)).get("content").asText());
    }

    private static String generateReviewReport(OpenAiClient client, String requirements, String designDoc, String apiCode, String tests) throws Exception {
        String prompt = "You are a staff engineer reviewing the following backend artifact bundle.\n\nRequirements:\n" + requirements + "\n\nDesign Document:\n" + designDoc + "\n\nAPI Code:\n" + apiCode + "\n\nTests:\n" + tests + "\n\nProduce a review report (Markdown) with:\n- Summary\n- What was done well\n- Risks and concerns\n- Actionable recommendations\n- Pass/needs-work verdict\n\nRespond with Markdown only.";
        ObjectNode payload = client.buildMessages(prompt);
        payload.put("max_tokens", 1500);
        payload.put("temperature", 0.2);
        return client.extractMessage(client.chatCompletion(payload)).get("content").asText();
    }

    private static String stripMarkdownFences(String text) {
        text = text.trim();
        if (text.startsWith("```")) {
            String[] lines = text.split("\n");
            int start = lines[0].startsWith("```") ? 1 : 0;
            int end = lines.length;
            if (end > 0 && lines[end - 1].startsWith("```")) end--;
            text = String.join("\n", java.util.Arrays.copyOfRange(lines, start, end));
        }
        return text.trim();
    }

    private static Path writeArtifact(Path dir, String name, String content) throws Exception {
        Path path = dir.resolve(name);
        Files.writeString(path, content);
        return path;
    }
}
