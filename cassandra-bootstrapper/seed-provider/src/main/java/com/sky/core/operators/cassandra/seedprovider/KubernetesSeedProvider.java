package com.sky.core.operators.cassandra.seedprovider;

import com.sky.core.operators.cassandra.seedprovider.crd.Cassandra;
import com.sky.core.operators.cassandra.seedprovider.crd.CassandraList;
import com.sky.core.operators.cassandra.seedprovider.crd.DoneableCassandra;
import com.sky.core.operators.cassandra.seedprovider.crd.Rack;
import io.fabric8.kubernetes.api.model.apiextensions.CustomResourceDefinition;
import io.fabric8.kubernetes.api.model.apiextensions.CustomResourceDefinitionBuilder;
import io.fabric8.kubernetes.api.model.apiextensions.CustomResourceDefinitionNamesBuilder;
import io.fabric8.kubernetes.api.model.apiextensions.CustomResourceDefinitionSpecBuilder;
import io.fabric8.kubernetes.client.DefaultKubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClient;
import org.apache.cassandra.locator.SeedProvider;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.net.InetAddress;
import java.net.UnknownHostException;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

import static java.lang.String.format;

public class KubernetesSeedProvider implements SeedProvider {
    private static final Logger logger = LoggerFactory.getLogger(KubernetesSeedProvider.class);

    private final KubernetesClient client;
    private final PodNameDnsResolver podNameDnsResolver;
    private final Map<String, String> config;

    // used by cassandra at runtime during initialisation
    @SuppressWarnings("unused")
    public KubernetesSeedProvider(final Map<String, String> config) {
        this(new DefaultKubernetesClient(), new PodNameDnsResolver(), config);
    }

    KubernetesSeedProvider(final KubernetesClient client, final PodNameDnsResolver podNameDnsResolver, final Map<String, String> config) {
        this.client = client;
        this.podNameDnsResolver = podNameDnsResolver;
        this.config = config;
    }

    @Override
    public List<InetAddress> getSeeds() {
        final String ns = config.get("namespace");
        final String clusterName = config.get("clusterName");

        if (ns == null) {
            throw new RuntimeException("Missing mandatory namespace parameter in Cassandra config");
        }

        if (clusterName == null) {
            throw new RuntimeException("Missing mandatory clusterName parameter in Cassandra config");
        }

        Cassandra cassandra = client.customResources(cassandraCrd(), Cassandra.class, CassandraList.class, DoneableCassandra.class)
                .inNamespace(ns)
                .withName(clusterName)
                .get();
        if (cassandra == null) {
            throw new RuntimeException(format("Unable to find cassandra resource definition for %s.%s - Resource may have been deleted", ns, clusterName));
        }

        final Rack[] allRacks = cassandra.getSpec().getRacks();

        SeedIdentifier seedIdentifier = allRacks.length > 1 ?
                new MultiRackSeedIdentifier(clusterName, ns, allRacks) :
                new SingleRackSeedIdentifier(clusterName, ns, allRacks[0]);
        return seedIdentifier.determineSeeds();
    }

    private CustomResourceDefinition cassandraCrd() {
        return new CustomResourceDefinitionBuilder()
                .withSpec(new CustomResourceDefinitionSpecBuilder()
                        .withGroup("core.sky.uk")
                        .withNames(new CustomResourceDefinitionNamesBuilder().withPlural("cassandras").build())
                        .withVersion("v1alpha1")
                        .withScope("Namespaced")
                        .build())
                .build();
    }

    abstract private class SeedIdentifier {
        private final ClusterDomainDiscoverer clusterDomainDiscoverer;
        private final String clusterName;
        private final String namespace;

        SeedIdentifier(String clusterName, String namespace) {
            this.clusterName = clusterName;
            this.namespace = namespace;
            this.clusterDomainDiscoverer = new ClusterDomainDiscoverer(podNameDnsResolver);
        }

        abstract long maxSeeds();

        abstract String podName(String clusterName, int index);

        List<InetAddress> determineSeeds() {
            final String clusterDomain;
            try {
                clusterDomain = clusterDomainDiscoverer.getClusterDomain();
            } catch (UnknownHostException e) {
                throw new RuntimeException("Unable to lookup cluster domain", e);
            }

            final List<InetAddress> seedAddresses = new ArrayList<>();
            for (int i = 0; i < maxSeeds(); i++) {
                String hostname = format("%s.%s.%s.svc.%s", podName(clusterName, i), clusterName, namespace, clusterDomain);
                try {
                    seedAddresses.add(podNameDnsResolver.resolve(hostname));
                } catch (UnknownHostException e) {
                    logger.warn(format("Unable to resolve pod ip from hostname: %s", hostname));
                }
            }

            return seedAddresses;
        }
    }

    private class SingleRackSeedIdentifier extends SeedIdentifier {
        private final Rack rack;

        SingleRackSeedIdentifier(String clusterName, String namespace, Rack rack) {
            super(clusterName, namespace);
            this.rack = rack;
        }

        @Override
        public long maxSeeds() {
            return Math.max(Math.min(rack.getReplicas() / 2, 3), 1);
        }

        @Override
        String podName(String clusterName, int podIndex) {
            return format("%s-%s-%d", clusterName, rack.getName(), podIndex);
        }
    }

    private class MultiRackSeedIdentifier extends SeedIdentifier {
        private static final int MAX_SEEDS_PER_RACK = 1;
        private final Rack[] allRacks;

        MultiRackSeedIdentifier(String clusterName, String namespace, Rack[] allRacks) {
            super(clusterName, namespace);
            this.allRacks = allRacks;
        }

        @Override
        public long maxSeeds() {
            return allRacks.length * MAX_SEEDS_PER_RACK;
        }

        @Override
        String podName(String clusterName, int rackIndex) {
            return format("%s-%s-0", clusterName, allRacks[rackIndex].getName());
        }
    }

}
