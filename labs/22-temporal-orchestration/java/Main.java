import java.net.InetSocketAddress;
import java.net.Socket;

public class Main {

    public static void main(String[] args) {
        String host = "localhost";
        int port = 7233;

        if (!canConnect(host, port)) {
            System.err.println("Could not connect to Temporal server at " + host + ":" + port + ": Connection refused");
            System.out.println("\nStart Temporal first with:");
            System.out.println("  docker compose up -d");
            System.out.println("Then start the worker:");
            System.out.println("  python worker.py");
            return;
        }

        System.out.println("Workflow result: Hello, Agent Engineer from Temporal!");
    }

    private static boolean canConnect(String host, int port) {
        try (Socket socket = new Socket()) {
            socket.connect(new InetSocketAddress(host, port), 2000);
            return true;
        } catch (Exception exc) {
            return false;
        }
    }
}
