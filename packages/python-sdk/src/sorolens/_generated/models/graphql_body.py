from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.graphql_body_variables import GraphqlBodyVariables


T = TypeVar("T", bound="GraphqlBody")


@_attrs_define
class GraphqlBody:
    """
    Attributes:
        query (str): GraphQL query document.
        operation_name (str | Unset): Name of the operation to execute, for multi-operation documents.
        variables (GraphqlBodyVariables | Unset): Values for the query's GraphQL variables.
    """

    query: str
    operation_name: str | Unset = UNSET
    variables: GraphqlBodyVariables | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        query = self.query

        operation_name = self.operation_name

        variables: dict[str, Any] | Unset = UNSET
        if not isinstance(self.variables, Unset):
            variables = self.variables.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "query": query,
            }
        )
        if operation_name is not UNSET:
            field_dict["operationName"] = operation_name
        if variables is not UNSET:
            field_dict["variables"] = variables

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.graphql_body_variables import (
            GraphqlBodyVariables,
        )

        d = dict(src_dict)
        query = d.pop("query")

        operation_name = d.pop("operationName", UNSET)

        _variables = d.pop("variables", UNSET)
        variables: GraphqlBodyVariables | Unset
        if isinstance(_variables, Unset):
            variables = UNSET
        else:
            variables = GraphqlBodyVariables.from_dict(_variables)

        graphql_body = cls(
            query=query,
            operation_name=operation_name,
            variables=variables,
        )

        graphql_body.additional_properties = d
        return graphql_body

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
