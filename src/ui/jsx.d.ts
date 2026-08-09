declare namespace JSX {
	type Element = Node;

	interface ElementChildrenAttribute {
		children: unknown;
	}

	interface IntrinsicElements {
		[elementName: string]: Record<string, unknown>;
	}
}
