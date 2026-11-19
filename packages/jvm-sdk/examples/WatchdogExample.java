// Walk every contract and every watchdog event through the supported client.
//
// Compile against the published jar (or `gradle jar`), then:
//
//   java -cp build/libs/sorolens-jvm-0.1.0.jar:examples WatchdogExample \
//        sl_api_key CDLZ...
//
// The example is intentionally outside src/ so it never ships in the artifact.
import com.fasterxml.jackson.databind.JsonNode;
import xyz.sorolens.sdk.SorolensClient;

public final class WatchdogExample {

    public static void main(String[] args) {
        String apiKey = args.length > 0 ? args[0] : null;
        String contractId = args.length > 1 ? args[1] : "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC";

        try (SorolensClient client = SorolensClient.builder().apiKey(apiKey).build()) {
            System.out.printf("tracked contracts: %s%n", client.globalStats().path("tracked_contracts"));

            long contracts = 0;
            for (JsonNode contract : client.contracts(100)) {
                contracts++;
                System.out.printf("  %s (%s)%n", contract.path("id").asText(), contract.path("status").asText());
            }

            long events = 0;
            for (JsonNode event : client.events(contractId, 100)) {
                events++;
                System.out.printf("  event %s ledger=%d%n", event.path("id").asText(), event.path("ledger").asInt());
            }
            System.out.printf("walked %d contracts and %d events%n", contracts, events);
        }
    }

    private WatchdogExample() {}
}
