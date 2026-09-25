from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast

if TYPE_CHECKING:
  from ..models.error_error_type_0 import ErrorErrorType0
  from ..models.error_error_type_1 import ErrorErrorType1





T = TypeVar("T", bound="Error")



@_attrs_define
class Error:
    """ 
        Attributes:
            error (ErrorErrorType0 | ErrorErrorType1):
     """

    error: ErrorErrorType0 | ErrorErrorType1
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        from ..models.error_error_type_0 import ErrorErrorType0 # noqa: PLC0415
        from ..models.error_error_type_1 import ErrorErrorType1 # noqa: PLC0415
        error: dict[str, Any]
        if isinstance(self.error, ErrorErrorType0):
            error = self.error.to_dict()
        else:
            error = self.error.to_dict()



        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "error": error,
        })

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.error_error_type_0 import ErrorErrorType0 # noqa: PLC0415
        from ..models.error_error_type_1 import ErrorErrorType1 # noqa: PLC0415
        d = dict(src_dict)
        def _parse_error(data: object) -> ErrorErrorType0 | ErrorErrorType1:
            try:
                if not isinstance(data, dict):
                    raise TypeError()
                error_type_0 = ErrorErrorType0.from_dict(data)



                return error_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            if not isinstance(data, dict):
                raise TypeError()
            error_type_1 = ErrorErrorType1.from_dict(data)



            return error_type_1

        error = _parse_error(d.pop("error"))


        error = cls(
            error=error,
        )


        error.additional_properties = d
        return error

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
