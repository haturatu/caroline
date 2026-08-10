import type { RenderActions } from "../../shared/types";

let actions: RenderActions = {};

export function setRenderActions(nextActions: RenderActions): void {
	actions = nextActions;
}

export function getRenderActions(): RenderActions {
	return actions;
}
