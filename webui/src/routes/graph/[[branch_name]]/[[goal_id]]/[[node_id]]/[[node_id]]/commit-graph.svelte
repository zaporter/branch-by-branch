				setTimeout(() => {
					if (!renderer) return;
					// Use Sigma's built-in method to focus on the node
					const camera = renderer.getCamera();
					const nodeAttrs = graphObject.getNodeAttributes(nodeId);
					const nodeDisplayPosition = renderer.graphToViewport({ x: nodeAttrs.x, y: nodeAttrs.y });
					camera.animate(nodeDisplayPosition, {
						duration: 500
					});
				}, 100); 