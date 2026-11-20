from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.contract_event_schema import ContractEventSchema
    from ..models.contract_event_schemas_response_declared_spec import (
        ContractEventSchemasResponseDeclaredSpec,
    )


T = TypeVar("T", bound="ContractEventSchemasResponse")


@_attrs_define
class ContractEventSchemasResponse:
    """
    Attributes:
        contract_id (str):
        event_schemas (list[ContractEventSchema]):
        declared_spec (ContractEventSchemasResponseDeclaredSpec | Unset): Declared SEP-48 interface spec if available.
    """

    contract_id: str
    event_schemas: list[ContractEventSchema]
    declared_spec: ContractEventSchemasResponseDeclaredSpec | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        event_schemas = []
        for event_schemas_item_data in self.event_schemas:
            event_schemas_item = event_schemas_item_data.to_dict()
            event_schemas.append(event_schemas_item)

        declared_spec: dict[str, Any] | Unset = UNSET
        if not isinstance(self.declared_spec, Unset):
            declared_spec = self.declared_spec.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "event_schemas": event_schemas,
            }
        )
        if declared_spec is not UNSET:
            field_dict["declared_spec"] = declared_spec

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.contract_event_schema import ContractEventSchema
        from ..models.contract_event_schemas_response_declared_spec import (
            ContractEventSchemasResponseDeclaredSpec,
        )

        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        event_schemas = []
        _event_schemas = d.pop("event_schemas")
        for event_schemas_item_data in _event_schemas:
            event_schemas_item = ContractEventSchema.from_dict(event_schemas_item_data)

            event_schemas.append(event_schemas_item)

        _declared_spec = d.pop("declared_spec", UNSET)
        declared_spec: ContractEventSchemasResponseDeclaredSpec | Unset
        if isinstance(_declared_spec, Unset):
            declared_spec = UNSET
        else:
            declared_spec = ContractEventSchemasResponseDeclaredSpec.from_dict(
                _declared_spec
            )

        contract_event_schemas_response = cls(
            contract_id=contract_id,
            event_schemas=event_schemas,
            declared_spec=declared_spec,
        )

        contract_event_schemas_response.additional_properties = d
        return contract_event_schemas_response

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
