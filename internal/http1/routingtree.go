package http1

import (
	"errors"
	"fmt"
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

func (tree *RoutingTree[T]) Find(key string) (*T, error) {
	components := strings.Split(key, "/")
	if components[0] != "" {
		return nil, errors.New("URL path does not start with '/'")
	}
	components = components[1:]

	currentNode := tree.root
	lastPrefixHandler := currentNode.prefixRouteHandler

	for idx, component := range components {
		key, err := extractKey(component)
		if err != nil {
			return nil, err
		}

		if currentNode.prefixRouteHandler != nil {
			lastPrefixHandler = currentNode.prefixRouteHandler
		}

		if idx+1 < len(components) {
			nextComponent := components[idx+1]
			if nextComponent == "" {
				if child := currentNode.children[key]; child != nil {
					if child.prefixRouteHandler != nil {
						return child.prefixRouteHandler, nil
					}
				}
				if lastPrefixHandler != nil {
					return lastPrefixHandler, nil
				}
				return nil, errors.New("could not match path to a registered route")
			}
			if child := currentNode.children[key]; child != nil {
				currentNode = child
			} else {
				if lastPrefixHandler != nil {
					return lastPrefixHandler, nil
				}
				return nil, errors.New("could not match path to a registered route")
			}
		} else {
			if child := currentNode.children[key]; child != nil {
				if child.exactPathHandler != nil {
					return child.exactPathHandler, nil
				}
				if child.prefixRouteHandler != nil {
					return child.prefixRouteHandler, nil
				}
			}
			if lastPrefixHandler != nil {
				return lastPrefixHandler, nil
			}
			return nil, errors.New("could not match path to a registered route")
		}
	}
	return nil, errors.New("could not match path to a registered route")
}

func extractKey(component string) (RoutingKey, error) {
	var key RoutingKey

	if strings.HasPrefix(component, "{") && strings.HasSuffix(component, "}") {
		component = component[1 : len(component)-1]
		if len(component) == 0 {
			return RoutingKey{}, fmt.Errorf("URL path contains empty placeholder %v", key)
		}
		key = RoutingKey{isParameter: true, match: component}
	} else {
		key = RoutingKey{isParameter: false, match: component}
	}
	return key, nil
}
