<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Domain\Factory;

use App\Domain\Enums\RoleEnums;
use App\Domain\ValueObject\RightsDescribe;

class RightsDescribeFactory
{
    /**
     * Проставляет флаги наличия определенных прав
     *
     * @param string[] $roles
     * @return RightsDescribe
     */
    public function byRoles(array $roles): RightsDescribe
    {
        $isSuperAdmin = false;

        if (in_array(RoleEnums::ROLE_SUPER_ADMIN->value, $roles, true)) {
            $isSuperAdmin = true;
        }

        return new RightsDescribe($isSuperAdmin);
    }

}
