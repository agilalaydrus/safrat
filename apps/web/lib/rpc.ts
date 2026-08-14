import { createClient } from "@connectrpc/connect";
import { AccommodationService } from "@hajj-saas/proto-gen/hajj/v1/accommodation_connect";
import { AgentService } from "@hajj-saas/proto-gen/hajj/v1/agent_connect";
import { OperatorService } from "@hajj-saas/proto-gen/hajj/v1/operator_connect";
import { PilgrimService } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_connect";
import { ProductService } from "@hajj-saas/proto-gen/hajj/v1/product_connect";
import { SeasonService } from "@hajj-saas/proto-gen/hajj/v1/season_connect";
import { TransportService } from "@hajj-saas/proto-gen/hajj/v1/transport_connect";
import { transport } from "./transport";

export const operatorClient = createClient(OperatorService, transport);
export const pilgrimClient = createClient(PilgrimService, transport);
export const seasonClient = createClient(SeasonService, transport);
export const accommodationClient = createClient(AccommodationService, transport);
export const transportClient = createClient(TransportService, transport);
export const productClient = createClient(ProductService, transport);
export const agentClient = createClient(AgentService, transport);
