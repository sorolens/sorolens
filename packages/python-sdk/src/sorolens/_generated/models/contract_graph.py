from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.contract_graph_edges_type_0_item import ContractGraphEdgesType0Item
    from ..models.contract_graph_nodes_type_0_item import ContractGraphNodesType0Item


T = TypeVar("T", bound="ContractGraph")


@_attrs_define
class ContractGraph:
    """
    Attributes:
        nodes (list[ContractGraphNodesType0Item] | None):
        edges (list[ContractGraphEdgesType0Item] | None):
    """

    nodes: list[ContractGraphNodesType0Item] | None
    edges: list[ContractGraphEdgesType0Item] | None
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        nodes: list[dict[str, Any]] | None
        if isinstance(self.nodes, list):
            nodes = []
            for nodes_type_0_item_data in self.nodes:
                nodes_type_0_item = nodes_type_0_item_data.to_dict()
                nodes.append(nodes_type_0_item)

        else:
            nodes = self.nodes

        edges: list[dict[str, Any]] | None
        if isinstance(self.edges, list):
            edges = []
            for edges_type_0_item_data in self.edges:
                edges_type_0_item = edges_type_0_item_data.to_dict()
                edges.append(edges_type_0_item)

        else:
            edges = self.edges

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "nodes": nodes,
                "edges": edges,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.contract_graph_edges_type_0_item import (
            ContractGraphEdgesType0Item,
        )
        from ..models.contract_graph_nodes_type_0_item import (
            ContractGraphNodesType0Item,
        )

        d = dict(src_dict)

        def _parse_nodes(data: object) -> list[ContractGraphNodesType0Item] | None:
            if data is None:
                return data
            try:
                if not isinstance(data, list):
                    raise TypeError()
                nodes_type_0 = []
                _nodes_type_0 = data
                for nodes_type_0_item_data in _nodes_type_0:
                    nodes_type_0_item = ContractGraphNodesType0Item.from_dict(
                        nodes_type_0_item_data
                    )

                    nodes_type_0.append(nodes_type_0_item)

                return nodes_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(list[ContractGraphNodesType0Item] | None, data)

        nodes = _parse_nodes(d.pop("nodes"))

        def _parse_edges(data: object) -> list[ContractGraphEdgesType0Item] | None:
            if data is None:
                return data
            try:
                if not isinstance(data, list):
                    raise TypeError()
                edges_type_0 = []
                _edges_type_0 = data
                for edges_type_0_item_data in _edges_type_0:
                    edges_type_0_item = ContractGraphEdgesType0Item.from_dict(
                        edges_type_0_item_data
                    )

                    edges_type_0.append(edges_type_0_item)

                return edges_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(list[ContractGraphEdgesType0Item] | None, data)

        edges = _parse_edges(d.pop("edges"))

        contract_graph = cls(
            nodes=nodes,
            edges=edges,
        )

        contract_graph.additional_properties = d
        return contract_graph

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
