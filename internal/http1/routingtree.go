package http1

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type RoutingNode[T any] struct {
	exactPathHandler   *T
	prefixRouteHandler *T
	children           map[RoutingKey]*RoutingNode[T]
}

type RoutingKey struct {
	isParameter bool
	match       string
}

type RoutingTree[T any] struct {
	root *RoutingNode[T]
}

func NewRoutingTree[T any]() *RoutingTree[T] {
	rootNode := &RoutingNode[T]{exactPathHandler: nil, prefixRouteHandler: nil, children: make(map[RoutingKey]*RoutingNode[T])}
	return &RoutingTree[T]{root: rootNode}
}

func NewRoutingNode[T any]() *RoutingNode[T] {
	return &RoutingNode[T]{exactPathHandler: nil, prefixRouteHandler: nil, children: make(map[RoutingKey]*RoutingNode[T])}
}

func (tree *RoutingTree[T]) Insert(key string, handler T) error {
	components := strings.Split(key, "/")
	if components[0] != "" {
		return errors.New("URL path does not start with '/'")
	}
	components = components[1:]

	currentNode := tree.root

	for idx, component := range components {
		key, err := extractKey(component)
		if err != nil {
			return err
		}

		if component == "" {
			return fmt.Errorf("URL path contains empty path segment %q", component)
		}

		if idx+1 < len(components) {
			nextComponent := components[idx+1]
			if nextComponent == "" {
				if child := currentNode.children[key]; child != nil {
					child.prefixRouteHandler = &handler
				} else {
					nextNode := NewRoutingNode[T]()
					nextNode.prefixRouteHandler = &handler
					currentNode.children[key] = nextNode
				}
				return nil
			}
			if child := currentNode.children[key]; child != nil {
				currentNode = child
			} else {
				nextNode := NewRoutingNode[T]()
				currentNode.children[key] = nextNode
				currentNode = nextNode
			}
		} else {
			if child := currentNode.children[key]; child != nil {
				child.exactPathHandler = &handler
			} else {
				nextNode := NewRoutingNode[T]()
				nextNode.exactPathHandler = &handler
				currentNode.children[key] = nextNode
			}
			return nil
		}
	}
	return nil
}

func (tree *RoutingTree[T]) Find(key string) (*T, map[string]string, error) {
	components := strings.Split(key, "/")
	if components[0] != "" {
		return nil, nil, errors.New("URL path does not start with '/'")
	}
	components = components[1:]
	args := make(map[string]string)

	currentNode := tree.root
	lastPrefixHandler := currentNode.prefixRouteHandler

	for idx, component := range components {
		key, err := extractKey(component)
		if err != nil {
			return nil, nil, err
		}

		if currentNode.prefixRouteHandler != nil {
			lastPrefixHandler = currentNode.prefixRouteHandler
		}

		if idx+1 < len(components) {
			nextComponent := components[idx+1]
			if nextComponent == "" {
				if child := currentNode.children[key]; child != nil {
					if child.prefixRouteHandler != nil {
						return child.prefixRouteHandler, args, nil
					}
				}

				if key := findParameterChild(currentNode); key != nil {
					child := currentNode.children[*key]
					args[key.match] = component
					if child.prefixRouteHandler != nil {
						return child.prefixRouteHandler, args, nil
					}
				}

				if lastPrefixHandler != nil {
					return lastPrefixHandler, args, nil
				}
				return nil, nil, errors.New("could not match path to a registered route")
			}
			if child := currentNode.children[key]; child != nil {
				currentNode = child
			} else if key := findParameterChild(currentNode); key != nil {
				currentNode = currentNode.children[*key]
				args[key.match] = component
			} else {
				if lastPrefixHandler != nil {
					return lastPrefixHandler, args, nil
				}
				return nil, nil, errors.New("could not match path to a registered route")
			}
		} else {
			if child := currentNode.children[key]; child != nil {
				if child.exactPathHandler != nil {
					return child.exactPathHandler, args, nil
				}
				if child.prefixRouteHandler != nil {
					return child.prefixRouteHandler, args, nil
				}
			} else if key := findParameterChild(currentNode); key != nil {
				args[key.match] = component
				child := currentNode.children[*key]
				if child.exactPathHandler != nil {
					return child.exactPathHandler, args, nil
				}
				if child.prefixRouteHandler != nil {
					return child.prefixRouteHandler, args, nil
				}
			}
			if lastPrefixHandler != nil {
				return lastPrefixHandler, args, nil
			}
			return nil, nil, errors.New("could not match path to a registered route")
		}
	}
	return nil, nil, errors.New("could not match path to a registered route")
}

func extractKey(component string) (RoutingKey, error) {
	key := RoutingKey{}

	if strings.HasPrefix(component, "{") && strings.HasSuffix(component, "}") {
		component = component[1 : len(component)-1]
		key.isParameter = true
	}
	if _, err := url.Parse(component); err != nil {
		return key, fmt.Errorf("URL path contains invalid characters %s", component)
	}
	if len(component) == 0 {
		return RoutingKey{}, fmt.Errorf("URL path contains empty path segment %s", component)
	}

	key.match = component
	return key, nil
}

func findParameterChild[T any](currentNode *RoutingNode[T]) *RoutingKey {
	for key := range currentNode.children {
		if key.isParameter {
			return &key
		}
	}
	return nil

}
