export type Child =
	| Node
	| string
	| number
	| boolean
	| null
	| undefined
	| Child[];

type Props = {
	children?: Child | Child[];
};

type Component = (props: Props) => Node;

const svgElements = new Set(["circle", "path", "svg"]);

function appendChildren(parent: Node, children: Child | Child[]): void {
	const list = Array.isArray(children) ? children : [children];
	for (const child of list) {
		if (Array.isArray(child)) {
			appendChildren(parent, child);
			continue;
		}
		if (
			child === null ||
			child === undefined ||
			child === false ||
			child === true
		)
			continue;
		parent.appendChild(
			child instanceof Node ? child : document.createTextNode(String(child)),
		);
	}
}

function setProperty(element: Element, name: string, value: unknown): void {
	const attribute =
		name === "className" ? "class" : name === "htmlFor" ? "for" : name;
	if (value === false || value === null || value === undefined) return;
	if (value === true) {
		element.setAttribute(attribute, "");
		return;
	}
	element.setAttribute(attribute, String(value));
}

export function h(
	tag: string | Component,
	props: Record<string, unknown> | null,
	...children: Child[]
): Node {
	if (typeof tag === "function") {
		return tag({ ...(props ?? {}), children } as Props);
	}

	const element = svgElements.has(tag)
		? document.createElementNS("http://www.w3.org/2000/svg", tag)
		: document.createElement(tag);
	for (const [name, value] of Object.entries(props ?? {})) {
		if (name !== "children") setProperty(element, name, value);
	}
	appendChildren(element, children);
	return element;
}

export function Fragment(props: Props): DocumentFragment {
	const fragment = document.createDocumentFragment();
	appendChildren(fragment, props.children ?? []);
	return fragment;
}
